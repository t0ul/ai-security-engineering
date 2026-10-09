package server

import (
	"encoding/json"
	"net/http"
)

// mcpRegister adds/updates an operator-registered tool server (name + URL + declared tool
// manifest). CSRF + authz(write/mcp). The manifest must still be Approved (pinned) before a
// tool can be called, and a call only reaches the URL if its host is on the egress allow-list.
func (s *Server) mcpRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string   `json:"name"`
		URL   string   `json:"url"`
		Tools []string `json:"tools"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.MCP.Register(req.Name, req.URL, req.Tools); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "server": req.Name})
}

// mcpDelete removes an operator-registered tool server. CSRF + authz(write/mcp).
func (s *Server) mcpDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Server string `json:"server"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Server == "" {
		http.Error(w, "server required", http.StatusBadRequest)
		return
	}
	if err := s.MCP.Delete(req.Server); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "server": req.Server})
}

// mcpCall invokes a declared tool on a registered server through the egress-gated sandbox.
// It fails closed (kill switch, approval pin, manifest scope) in the registry; here the
// operator-initiated call is CSRF + authz(write/mcp) gated.
func (s *Server) mcpCall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Server string `json:"server"`
		Tool   string `json:"tool"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Server == "" || req.Tool == "" {
		http.Error(w, "server and tool required", http.StatusBadRequest)
		return
	}
	out, err := s.MCP.Call(r.Context(), req.Server, req.Tool)
	if err != nil {
		writeJSON(w, map[string]any{"ok": false, "refused": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "output": out})
}

// mcpList returns the MCP registry with pin status for the console.
func (s *Server) mcpList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"servers": s.MCP.List()})
}

// mcpApprove re-pins a server to its current manifest (operator rug-pull
// recovery). The capability gate (write/mcp) and CSRF already guard this route.
func (s *Server) mcpApprove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Server string `json:"server"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Server == "" {
		http.Error(w, "server required", http.StatusBadRequest)
		return
	}
	if err := s.MCP.Approve(req.Server); err != nil {
		http.Error(w, "approve failed: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "server": req.Server})
}
