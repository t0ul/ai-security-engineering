package server

import (
	"encoding/json"
	"net/http"
)

// GrammarRow is one entry of the grammar governance tab: the governed output GBNF
// for a model, with its version and content hash. A swapped grammar silently
// changes the output contract, so it is pinned like a prompt.
type GrammarRow struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	Hash    string `json:"hash"`
	Text    string `json:"text"`
}

func (s *Server) grammarList(w http.ResponseWriter, _ *http.Request) {
	var out []GrammarRow
	for _, v := range s.Grammars.List() {
		out = append(out, GrammarRow{Name: v.Name, Version: v.Version, Hash: short12(v.Hash), Text: v.Text})
	}
	writeJSON(w, map[string]any{"grammars": out})
}

// grammarActivate records a new governed grammar version. CSRF + authz(write/grammar).
func (s *Server) grammarActivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Text == "" {
		http.Error(w, "name and text required", http.StatusBadRequest)
		return
	}
	v := s.Grammars.Activate(req.Name, req.Text)
	writeJSON(w, map[string]any{"ok": true, "name": v.Name, "version": v.Version, "hash": short12(v.Hash)})
}

// grammarReset rolls a grammar back to its shipped default.
func (s *Server) grammarReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	v := s.Grammars.Reset(req.Name)
	writeJSON(w, map[string]any{"ok": true, "name": v.Name, "version": v.Version})
}
