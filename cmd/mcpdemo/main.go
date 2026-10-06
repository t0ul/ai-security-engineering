// Command mcpdemo shows the MCP security pillar end to end: an agent reaches a
// web_fetch tool only through the gustoms gateway (registry allowlist, manifest
// pin / rug-pull defense, per-tool authorization), and the tool itself screens
// every target through netpolicy (M16: default-deny egress, no loopback/private/
// IMDS). Every decision is recorded to gledger.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/t0ul/ai-security-engineering/mcp"
	"github.com/t0ul/ai-security-engineering/netpolicy"
	"github.com/t0ul/gledger"
	"github.com/t0ul/gustoms"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mcpdemo:", err)
		os.Exit(1)
	}
}

func run() error {
	audit, err := gledger.Open(filepath.Join("controlplane", "logs", "mcp.jsonl"), "mcpdemo")
	if err != nil {
		return err
	}

	// A legitimate, allow-listed target the browse tool may reach.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("<html>search result</html>"))
	}))
	defer target.Close()

	policy := netpolicy.Policy{
		Allow:         []string{"127.0.0.1", "intranet.test"},
		AllowLoopback: true, // dev: the demo target is on loopback
		Resolve: func(host string) ([]net.IP, error) {
			if host == "intranet.test" { // an allow-listed name that rebinds internally
				return []net.IP{net.ParseIP("10.0.0.9")}, nil
			}
			return net.LookupIP(host)
		},
	}
	mcpSrv := httptest.NewServer(mcp.NewServer(mcp.WebFetchTool(policy, 0)))
	defer mcpSrv.Close()

	gw := gustoms.New(
		gustoms.WithServer(gustoms.Server{Name: "search", Client: &mcp.HTTPClient{URL: mcpSrv.URL}, AllowedTools: []string{"web_fetch"}}),
		gustoms.WithAuthorizer(func(caller, _, _ string) bool { return caller == "agent" }),
		gustoms.WithAuditor(audit),
	)

	call := func(caller, server, tool, url string) {
		args := map[string]any{}
		if url != "" {
			args["url"] = url
		}
		out, err := gw.Call(context.Background(), gledger.NewTraceID(), caller, server, tool, args)
		label := fmt.Sprintf("%s -> %s/%s %s", caller, server, tool, url)
		if err != nil {
			fmt.Printf("  BLOCKED  %s\n           %v\n", label, err)
			return
		}
		fmt.Printf("  ALLOWED  %s\n           %v\n", label, out)
	}

	fmt.Println("== MCP gateway + egress containment demo ==")
	fmt.Println("1) allow-listed fetch via the gateway:")
	call("agent", "search", "web_fetch", target.URL)
	fmt.Println("2) SSRF to cloud metadata (IMDS):")
	call("agent", "search", "web_fetch", "http://169.254.169.254/latest/meta-data/")
	fmt.Println("3) non-allow-listed host (default-deny):")
	call("agent", "search", "web_fetch", "https://evil.example/")
	fmt.Println("4) DNS-rebinding: allow-listed name resolving internal:")
	call("agent", "search", "web_fetch", "http://intranet.test/")
	fmt.Println("5) tool not permitted on this server:")
	call("agent", "search", "shell_exec", "")
	fmt.Println("6) unknown (non-allow-listed) MCP server:")
	call("agent", "rogue", "web_fetch", target.URL)
	fmt.Println("7) unauthorized caller:")
	call("attacker", "search", "web_fetch", target.URL)

	ok, n := audit.Verify()
	fmt.Printf("\naudit: chain_ok=%t records=%d\n", ok, n)
	return nil
}
