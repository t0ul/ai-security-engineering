package webapp

import (
	"encoding/json"
	"net/http"
)

// BundleInfo is one saved known-good snapshot for the console (C10).
type BundleInfo struct {
	Label string `json:"label"`
	At    string `json:"at"`
}

func (s *Server) bundlesList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{"bundles": s.BundleList()})
}

// bundlesSave snapshots the whole governed plane under a label. CSRF +
// authz(write/bundle).
func (s *Server) bundlesSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Label string `json:"label"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Label == "" {
		http.Error(w, "label required", http.StatusBadRequest)
		return
	}
	if err := s.BundleSave(req.Label); err != nil {
		http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "label": req.Label})
}

// bundlesApply rolls the entire governed plane back to a saved snapshot. CSRF +
// authz(write/bundle).
func (s *Server) bundlesApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Label string `json:"label"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Label == "" {
		http.Error(w, "label required", http.StatusBadRequest)
		return
	}
	if err := s.BundleApply(req.Label); err != nil {
		http.Error(w, "rollback failed: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "label": req.Label})
}
