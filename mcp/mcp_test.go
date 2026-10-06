package mcp_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/t0ul/ai-security-engineering/mcp"
	"github.com/t0ul/ai-security-engineering/netpolicy"
	"github.com/t0ul/gustoms"
)

// gatewayOver wires an MCP web_fetch server behind gustoms and returns the
// gateway plus the allowed target server's URL.
func gatewayOver(t *testing.T) (*gustoms.Gateway, string) {
	t.Helper()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("hello from target"))
	}))
	t.Cleanup(target.Close)

	policy := netpolicy.Policy{
		Allow:         []string{"127.0.0.1", "metadata.test"},
		AllowLoopback: true, // dev: reach the loopback test target
		Resolve: func(host string) ([]net.IP, error) {
			switch host {
			case "metadata.test":
				return []net.IP{net.ParseIP("169.254.169.254")}, nil
			default:
				return net.LookupIP(host)
			}
		},
	}
	mcpSrv := httptest.NewServer(mcp.NewServer(mcp.WebFetchTool(policy, nil, 0)))
	t.Cleanup(mcpSrv.Close)

	gw := gustoms.New(gustoms.WithServer(gustoms.Server{
		Name: "search", Client: &mcp.HTTPClient{URL: mcpSrv.URL}, AllowedTools: []string{"web_fetch"},
	}))
	return gw, target.URL
}

func TestWebFetchThroughGatewayAllowed(t *testing.T) {
	gw, targetURL := gatewayOver(t)
	out, err := gw.Call(context.Background(), "t", "agent", "search", "web_fetch", map[string]any{"url": targetURL})
	if err != nil {
		t.Fatalf("allowed fetch failed: %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok || m["status"].(float64) != 200 || !strings.Contains(m["body"].(string), "hello from target") {
		t.Fatalf("unexpected result: %v", out)
	}
}

func TestWebFetchBlocksIMDS(t *testing.T) {
	gw, _ := gatewayOver(t)
	_, err := gw.Call(context.Background(), "t", "agent", "search", "web_fetch", map[string]any{"url": "http://metadata.test/latest/meta-data/"})
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("IMDS fetch should be blocked by netpolicy, got %v", err)
	}
}

func TestWebFetchDefaultDeny(t *testing.T) {
	gw, _ := gatewayOver(t)
	_, err := gw.Call(context.Background(), "t", "agent", "search", "web_fetch", map[string]any{"url": "https://evil.example/"})
	if err == nil || !strings.Contains(err.Error(), "allowlist") {
		t.Fatalf("non-allowlisted host should be denied, got %v", err)
	}
}
