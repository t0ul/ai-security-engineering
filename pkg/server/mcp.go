package server

import (
	"encoding/json"
	"net/http"
)

// MCPServer is one row of the MCP governance tab (C6): a server the agent reaches
// through the gustoms gateway, its advertised + allow-listed tools, the approved
// pin vs the live manifest hash, and a plain-language status. Status is one of
// "pinned" (manifest matches the approved pin), "rug-pull" (manifest changed —
// calls refused until re-approval), "unapproved" (no pin yet, strict pinning
// refuses calls), "blocked" (kill switch is refusing all tool calls), or "error".
type MCPServer struct {
	Name    string   `json:"name"`
	Tools   []string `json:"tools"`
	Allowed []string `json:"allowed"`
	Pinned  string   `json:"pinned"`  // short approved manifest hash
	Current string   `json:"current"` // short live manifest hash
	Status  string   `json:"status"`
}

// mcpList returns the MCP registry with pin status for the console.
func (s *Server) mcpList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"servers": s.MCP()})
}

// mcpApprove re-pins a server to its current manifest (operator rug-pull
// recovery). The capability gate (write/mcp) and CSRF already guard this route.
func (s *Server) mcpApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
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
