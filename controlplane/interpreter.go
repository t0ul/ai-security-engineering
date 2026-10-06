package controlplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/t0ul/gledger"
)

// ForbiddenSignatures are the outbound-command denylist: a planner-chosen
// command containing any of these is blocked before it ever reaches the sandbox.
// This is the CaMeL outbound policy engine's coarse first gate; the MicroVM is
// the real isolation boundary behind it.
var ForbiddenSignatures = []string{"rm -rf", "nc -e", "mkfifo", "> /dev/tcp"}

// Interpreter validates a planner-chosen command against the policy denylist,
// then pushes it across the vsock boundary (bridged to host loopback) to the
// MicroVM for isolated detonation. Every decision is audited under the request's
// trace_id: the policy gate and the vsock result.
type Interpreter struct {
	MicroVMURL string // default http://127.0.0.1:5000 (vsock bridge; use 127.0.0.1, not localhost)
	Audit      *gledger.AuditLog
	HTTP       *http.Client
}

// NewInterpreter returns an interpreter targeting microVMURL (empty = default
// local bridge) and auditing to the given log.
func NewInterpreter(microVMURL string, audit *gledger.AuditLog) *Interpreter {
	if microVMURL == "" {
		microVMURL = "http://127.0.0.1:5000"
	}
	return &Interpreter{MicroVMURL: microVMURL, Audit: audit, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

type detonateRequest struct {
	Command string `json:"command"`
	TraceID string `json:"trace_id"`
}

type detonateResponse struct {
	Output  string `json:"output"`
	TraceID string `json:"trace_id"`
}

// ExecuteInSandbox applies the policy gate, then detonates the command in the
// MicroVM. A blocked command never leaves the host; a connection failure is
// reported, not fatal. The returned string is the sandbox output (or a status
// message prefixed to signal block/failure to the caller).
func (in *Interpreter) ExecuteInSandbox(ctx context.Context, traceID, command string) string {
	for _, sig := range ForbiddenSignatures {
		if strings.Contains(command, sig) {
			in.Audit.Emit(traceID, "policy", "gate", gledger.F{
				"decision": "block", "signature": sig, "command": command,
			})
			return fmt.Sprintf("[Interpreter] BLOCKED: malicious signature %q detected.", sig)
		}
	}
	in.Audit.Emit(traceID, "policy", "gate", gledger.F{"decision": "allow", "command": command})

	buf, _ := json.Marshal(detonateRequest{Command: command, TraceID: traceID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, in.MicroVMURL, bytes.NewReader(buf))
	if err != nil {
		in.Audit.Emit(traceID, "vsock", "detonate", gledger.F{"ok": false, "error": err.Error()})
		return fmt.Sprintf("[Interpreter] MicroVM request build failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := in.HTTP.Do(req)
	if err != nil {
		in.Audit.Emit(traceID, "vsock", "detonate", gledger.F{"ok": false, "error": err.Error()})
		return fmt.Sprintf("[Interpreter] MicroVM connection failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		in.Audit.Emit(traceID, "vsock", "detonate", gledger.F{"ok": false, "status": resp.StatusCode})
		return fmt.Sprintf("[Interpreter] MicroVM returned status %d", resp.StatusCode)
	}
	var data detonateResponse
	if err := json.Unmarshal(body, &data); err != nil {
		in.Audit.Emit(traceID, "vsock", "detonate", gledger.F{"ok": false, "error": "bad json"})
		return "[Interpreter] MicroVM returned unreadable response"
	}
	in.Audit.Emit(traceID, "vsock", "detonate", gledger.F{
		"ok": true, "out_len": len(data.Output), "vm_trace": data.TraceID,
	})
	return data.Output
}
