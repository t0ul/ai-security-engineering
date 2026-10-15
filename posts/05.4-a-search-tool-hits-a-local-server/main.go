// Command main is the runnable demo for lesson 05.4, a web-search tool that hits a
// local server.
//
// Run it with:
//
//	go run .
//
// The agent gains its first tool that reaches out on its own: a web search. A tool
// that fetches any URL it is given is a server-side request forgery waiting to
// happen, because the URL comes from the model, which read an untrusted email. This
// demo points a naive fetcher at the agent's own machine and watches it comply,
// then puts the real egress policy in front and watches it refuse, while still
// allowing a legitimate search host. Everything is local; nothing leaves the
// machine. Deterministic, no model.
package main

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"fmt"

	"github.com/t0ul/emaildrop/pkg/netpolicy"
)

// resolve is a fixed resolver so the demo is deterministic: a real search host
// resolves to a public address, and literal IPs resolve to themselves.
func resolve(host string) ([]net.IP, error) {
	if host == "search.example" {
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}
	if host == "rebind.example" {
		return []net.IP{net.ParseIP("127.0.0.1")}, nil // on the allowlist, but points inward
	}
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	return net.LookupIP(host)
}

// policy is the agent's egress guard: a default-deny allowlist that also refuses
// any host resolving to a loopback, link-local, or private address.
var policy = netpolicy.Policy{Allow: []string{"search.example", "rebind.example"}, Resolve: resolve}

func main() {
	// An internal service on the agent's own machine that must never be reachable
	// through a tool, standing in for an admin API or the cloud-metadata endpoint.
	var hits int
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		io.WriteString(w, "INTERNAL-SECRET: admin-token")
	}))
	defer internal.Close()

	landed := naiveReaches(internal.URL, &hits)
	blocked := policyRefuses(internal.URL, &hits)

	fmt.Println()
	if landed && blocked {
		fmt.Println("attack blocked (the receipt): the naive tool reaches inside; the gated tool refuses")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- before: a tool that fetches whatever it is given ----

// naiveFetch is the web-search tool I would write first: given a URL, fetch it.
func naiveFetch(rawURL string) (string, error) {
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Get(rawURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func naiveReaches(internalURL string, hits *int) bool {
	fmt.Println("== before: the naive search tool ==")
	*hits = 0
	body, err := naiveFetch(internalURL)
	if err != nil {
		fmt.Println("  fetch error:", err)
		return false
	}
	fmt.Printf("  asked to fetch %s\n  it returned:  %q (internal service hit %d time(s))\n", internalURL, body, *hits)
	return *hits > 0 // SSRF: the tool reached the machine's own internal service
}

// ---- after: every fetch passes the egress policy first ----

// gatedFetch checks the target against the egress policy before fetching, and
// refuses anything the policy rejects.
func gatedFetch(rawURL string) (string, error) {
	if err := policy.Check(rawURL); err != nil {
		return "", err
	}
	return naiveFetch(rawURL)
}

func policyRefuses(internalURL string, hits *int) bool {
	fmt.Println("\n== after: the egress policy in front of the tool ==")
	*hits = 0

	// The same internal target the naive tool reached.
	_, err := gatedFetch(internalURL)
	fmt.Printf("  loopback %s\n    refused: %v (internal service hit %d time(s))\n", internalURL, err, *hits)

	// The cloud-metadata address, the classic SSRF credential grab.
	_, errMeta := gatedFetch("http://169.254.169.254/latest/meta-data/")
	fmt.Printf("  metadata http://169.254.169.254/...\n    refused: %v\n", errMeta)

	// An allowlisted NAME that resolves to an internal IP: DNS rebinding. The
	// allowlist alone would pass it; resolving the host is what catches it.
	_, errRebind := gatedFetch("http://rebind.example/")
	fmt.Printf("  allowlisted-but-rebinding http://rebind.example/ -> 127.0.0.1\n    refused: %v\n", errRebind)

	// A legitimate search host is still allowed: the policy is an allowlist, not a wall.
	allowed := policy.Check("http://search.example/q=book+fair")
	fmt.Printf("  allowlisted http://search.example/...\n    allowed: %v\n", allowed == nil)

	return *hits == 0 && err != nil && errMeta != nil && errRebind != nil && allowed == nil
}

// ---- helpers ----

// newInternal is the throwaway internal service used by the tests.
func newInternal(hits *int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		*hits++
		io.WriteString(w, "INTERNAL-SECRET: admin-token")
	}))
}

var _ = strings.TrimSpace
