package controlplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// countingVM records how many times it was reached and echoes the argv it saw.
func countingVM(t *testing.T, hits *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		var req detonateRequest
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(detonateResponse{Output: "ran: " + strings.Join(req.Argv, " "), TraceID: "vm1"})
	}))
}

func callTool(t *testing.T, in *Interpreter, argv ...string) string {
	t.Helper()
	tool := SandboxExecTool(in)
	anyArgv := make([]any, len(argv))
	for i, a := range argv {
		anyArgv[i] = a
	}
	out, err := tool.Handler(context.Background(), map[string]any{"argv": anyArgv, "trace_id": "t"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	return out.(map[string]any)["output"].(string)
}

func TestSandboxToolAllowlistedReachesVM(t *testing.T) {
	audit, _ := tempAudit(t)
	hits := 0
	vm := countingVM(t, &hits)
	defer vm.Close()
	in := &Interpreter{MicroVMURL: vm.URL, Audit: audit, HTTP: vm.Client()}

	out := callTool(t, in, "uname", "-a")
	if hits != 1 {
		t.Fatalf("allowlisted command should reach the VM once, hits=%d", hits)
	}
	if !strings.Contains(out, "ran: uname -a") {
		t.Fatalf("expected sandbox output, got %q", out)
	}
}

func TestSandboxToolShellMetaRejectedBeforeVM(t *testing.T) {
	audit, _ := tempAudit(t)
	hits := 0
	vm := countingVM(t, &hits)
	defer vm.Close()
	in := &Interpreter{MicroVMURL: vm.URL, Audit: audit, HTTP: vm.Client()}

	// "ls" is allowlisted, but the argument carries a shell metacharacter: it
	// must be refused by argcheck before anything is detonated.
	out := callTool(t, in, "ls", "x; echo PWNED")
	if hits != 0 {
		t.Fatalf("a shell-meta argument must never reach the VM, hits=%d", hits)
	}
	if !strings.Contains(out, "BLOCKED") {
		t.Fatalf("expected BLOCKED, got %q", out)
	}
}

func TestSandboxFetchRoutesThroughVM(t *testing.T) {
	audit, _ := tempAudit(t)
	hits := 0
	vm := countingVM(t, &hits)
	defer vm.Close()
	in := &Interpreter{MicroVMURL: vm.URL, Audit: audit, HTTP: vm.Client()}
	tool := SandboxFetchTool(in)

	// Valid URL (with a query string, which the shell-meta gate would wrongly
	// reject) detonates vmfetch inside the VM.
	out, err := tool.Handler(context.Background(), map[string]any{"url": "https://example.com/a?b=1&c=2", "trace_id": "t"})
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("fetch should reach the VM once, hits=%d", hits)
	}
	if got := out.(map[string]any)["output"].(string); !strings.Contains(got, "ran: vmfetch https://example.com/a?b=1&c=2") {
		t.Fatalf("expected vmfetch argv in the VM, got %q", got)
	}

	// A non-http(s) URL is refused before any detonation.
	out, err = tool.Handler(context.Background(), map[string]any{"url": "file:///etc/passwd", "trace_id": "t"})
	if err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("a bad-scheme URL must not reach the VM, hits=%d", hits)
	}
	if got := out.(map[string]any)["output"].(string); !strings.Contains(got, "BLOCKED") {
		t.Fatalf("expected BLOCKED for file:// URL, got %q", got)
	}
}

func TestSandboxToolNonAllowlistedRejectedBeforeVM(t *testing.T) {
	audit, _ := tempAudit(t)
	hits := 0
	vm := countingVM(t, &hits)
	defer vm.Close()
	in := &Interpreter{MicroVMURL: vm.URL, Audit: audit, HTTP: vm.Client()}

	// `rm --recursive /` is not caught by the "rm -rf" blocklist signature, but
	// the allowlist refuses it because `rm` is not a permitted argv[0].
	out := callTool(t, in, "rm", "--recursive", "/")
	if hits != 0 {
		t.Fatalf("non-allowlisted command must never reach the VM, hits=%d", hits)
	}
	if !strings.Contains(out, "BLOCKED") {
		t.Fatalf("expected BLOCKED, got %q", out)
	}
}
