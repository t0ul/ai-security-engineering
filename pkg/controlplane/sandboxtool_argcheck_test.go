package controlplane

import (
	"context"
	"strings"
	"testing"
)

// TestWebFetchArgGate locks A6: the web_fetch tool validates its arguments before they
// cross into the sandbox — unknown args, injection-shaped urls, and a shell-meta trace_id
// are refused; a clean url + trace_id pass. Exercises the live tool handler.
func TestWebFetchArgGate(t *testing.T) {
	// Direct arg-shape checks.
	bad := []map[string]any{
		{"url": "https://ok.test/a", "evil": "x"},           // unknown arg
		{"url": ""},                                          // empty
		{"url": "https://ok.test/a\nrm -rf /"},               // control char in url
		{"url": "https://ok.test/a b"},                       // whitespace in url
		{"url": "https://ok.test/a", "trace_id": "t;reboot"}, // shell-meta trace_id
		{"url": 42},                                          // non-string url
	}
	for i, a := range bad {
		if err := fetchArgsOK(a); err == nil {
			t.Errorf("case %d must be refused: %v", i, a)
		}
	}
	for _, a := range []map[string]any{
		{"url": "https://schools.nyc.gov/a?b=1&c=2"}, // query string is fine
		{"url": "https://ok.test/a", "trace_id": "abc123"},
	} {
		if err := fetchArgsOK(a); err != nil {
			t.Errorf("clean args must pass, got %v for %v", err, a)
		}
	}

	// Through the real tool handler: a bad arg never reaches ExecuteFetch.
	in := &Interpreter{} // no MicroVMURL; if the gate failed to stop it, ExecuteFetch would try to dial
	tool := SandboxFetchTool(in)
	if _, err := tool.Handler(context.Background(), map[string]any{"url": "https://ok.test", "trace_id": "t;$(id)"}); err == nil || !strings.Contains(err.Error(), "argcheck") {
		t.Errorf("a shell-meta trace_id must be refused by the handler, got %v", err)
	}
}
