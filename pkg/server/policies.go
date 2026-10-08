package server

import (
	"encoding/json"
	"net/http"
)

// PolicyRow is one entry of the policies governance tab (C8): a named security
// allowlist (egress hosts, exec argv[0]s, ...) with its version and content hash.
// Version 0 is the shipped default; >0 is a governed activation.
type PolicyRow struct {
	Name    string   `json:"name"`
	Version int      `json:"version"`
	Hash    string   `json:"hash"` // short content hash
	Items   []string `json:"items"`
}

func (s *Server) policiesList(w http.ResponseWriter, _ *http.Request) {
	var out []PolicyRow
	for _, pv := range s.Policies.List() {
		out = append(out, PolicyRow{Name: pv.Name, Version: pv.Version, Hash: short12(pv.Hash), Items: pv.Items})
	}
	writeJSON(w, map[string]any{"policies": out})
}

// policiesActivate records a new governed version of an allowlist (versioned,
// hashed, audited, persisted). CSRF + authz(write/policy) guard this route. The
// deny-by-default enforcement (private/IMDS, argcheck) is unaffected.
func (s *Server) policiesActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name  string   `json:"name"`
		Items []string `json:"items"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	pv := s.Policies.Activate(req.Name, req.Items)
	writeJSON(w, map[string]any{"ok": true, "name": pv.Name, "version": pv.Version, "hash": short12(pv.Hash)})
}

// policiesReset rolls an allowlist back to its shipped default.
func (s *Server) policiesReset(w http.ResponseWriter, r *http.Request) {
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
	pv := s.Policies.Reset(req.Name)
	writeJSON(w, map[string]any{"ok": true, "name": pv.Name, "version": pv.Version})
}
