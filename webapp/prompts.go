package webapp

import (
	"encoding/json"
	"net/http"
)

// PromptRow is one entry of the prompts governance tab (C2): the active (or
// shipped-default) system prompt for a logical model, with its version and
// content hash. Version 0 is the shipped default; >0 is a governed activation.
type PromptRow struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	Hash    string `json:"hash"` // short content hash
	Text    string `json:"text"`
}

func short12(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// promptsList returns the prompt inventory for the console.
func (s *Server) promptsList(w http.ResponseWriter, _ *http.Request) {
	var out []PromptRow
	for _, pv := range s.Prompts.List() {
		out = append(out, PromptRow{Name: pv.Name, Version: pv.Version, Hash: short12(pv.Hash), Text: pv.Text})
	}
	writeJSON(w, map[string]any{"prompts": out})
}

// promptsActivate records a new governed version of a prompt (versioned, hashed,
// audited, persisted via the resolver's OnActivate). CSRF + authz(write/prompts)
// already guard this route.
func (s *Server) promptsActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Text == "" {
		http.Error(w, "name and text required", http.StatusBadRequest)
		return
	}
	pv := s.Prompts.Activate(req.Name, req.Text)
	writeJSON(w, map[string]any{"ok": true, "name": pv.Name, "version": pv.Version, "hash": short12(pv.Hash)})
}

// promptsReset rolls a prompt back to its shipped default (known-good).
func (s *Server) promptsReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	pv := s.Prompts.Reset(req.Name)
	writeJSON(w, map[string]any{"ok": true, "name": pv.Name, "version": pv.Version})
}
