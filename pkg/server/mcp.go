package server

import (
	"encoding/json"
	"net/http"
)

// mcpList returns the MCP registry with pin status for the console.
func (s *Server) mcpList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"servers": s.MCP()})
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
	if err := s.MCPApprove(req.Server); err != nil {
		http.Error(w, "approve failed: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "server": req.Server})
}
