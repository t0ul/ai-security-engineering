package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/t0ul/gledger"
)

// TestCommandsStreamPairsGateAndDetonate: the command stream reads the real tamper-evident
// audit log and pairs each policy/gate (the command) with its vsock/detonate (the outcome)
// by trace — showing the exact argv/URL, the decision, and the result, but never output.
func TestCommandsStreamPairsGateAndDetonate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	audit, err := gledger.Open(path, "test")
	if err != nil {
		t.Fatal(err)
	}

	tr1 := gledger.NewTraceID() // an allowed fetch the sandbox answered 403
	audit.Emit(tr1, "policy", "gate", gledger.F{"decision": "allow", "tool": "vmfetch", "url": "https://wx.test/api"})
	audit.Emit(tr1, "vsock", "detonate", gledger.F{"ok": false, "status": 403})

	tr2 := gledger.NewTraceID() // an allowed exec that ran
	audit.Emit(tr2, "policy", "gate", gledger.F{"decision": "allow", "argv": []string{"uname", "-a"}})
	audit.Emit(tr2, "vsock", "detonate", gledger.F{"ok": true, "out_len": 42})

	tr3 := gledger.NewTraceID() // a blocked command
	audit.Emit(tr3, "policy", "gate", gledger.F{"decision": "block", "reason": "argv[0] not allowed", "argv": []string{"curl", "evil"}})

	srv := httptest.NewServer(New(Config{AuditPath: path}).Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/commands")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var d struct {
		Commands []map[string]any `json:"commands"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		t.Fatal(err)
	}
	if len(d.Commands) != 3 {
		t.Fatalf("want 3 command rows, got %d: %v", len(d.Commands), d.Commands)
	}
	by := map[string]map[string]any{}
	for _, c := range d.Commands {
		by[c["command"].(string)] = c
	}
	if f := by["https://wx.test/api"]; f == nil || f["kind"] != "fetch" || f["decision"] != "allow" || f["outcome"] != "status 403" {
		t.Errorf("fetch row wrong: %v", f)
	}
	if e := by["uname -a"]; e == nil || e["kind"] != "exec" || e["outcome"] != "ok (42 bytes)" {
		t.Errorf("exec row wrong: %v", e)
	}
	if b := by["curl evil"]; b == nil || b["decision"] != "block" || b["outcome"] != "blocked" {
		t.Errorf("blocked row wrong: %v", b)
	}
}
