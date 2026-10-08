package server

import (
	"encoding/json"
	"net/http"
)

// ModelRow is one entry of the model-binding governance tab: the logical model a
// role routes to, with its version and content hash.
type ModelRow struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	Hash    string `json:"hash"`
	Model   string `json:"model"`
}

func (s *Server) modelsList(w http.ResponseWriter, _ *http.Request) {
	var out []ModelRow
	for _, v := range s.Models.List() {
		out = append(out, ModelRow{Name: v.Name, Version: v.Version, Hash: short12(v.Hash), Model: v.Model})
	}
	writeJSON(w, map[string]any{"models": out})
}

// modelsActivate records a new governed model binding for a role.
// CSRF + authz(write/model) guard this route.
func (s *Server) modelsActivate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Model string `json:"model"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Model == "" {
		http.Error(w, "name and model required", http.StatusBadRequest)
		return
	}
	v := s.Models.Activate(req.Name, req.Model)
	writeJSON(w, map[string]any{"ok": true, "name": v.Name, "version": v.Version, "hash": short12(v.Hash)})
}

// modelsReset rolls a role's model binding back to its shipped default.
func (s *Server) modelsReset(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	v := s.Models.Reset(req.Name)
	writeJSON(w, map[string]any{"ok": true, "name": v.Name, "version": v.Version})
}
