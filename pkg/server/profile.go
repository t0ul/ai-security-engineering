package server

import (
	"encoding/json"
	"net/http"

	"github.com/t0ul/ai-security-engineering/pkg/domain"
)

// loadProfile reads the household profile from the governed store (empty if none).
func (s *Server) loadProfile() domain.Profile {
	if s.ProfileLoad != nil {
		return s.ProfileLoad()
	}
	return domain.Profile{}
}

func (s *Server) profileGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.loadProfile())
}

// profileSave persists the household profile to the DB (host-only, no egress).
// CSRF + authz(write/profile) guard this route.
func (s *Server) profileSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if s.ProfileSave == nil {
		http.Error(w, "no profile store configured", http.StatusNotFound)
		return
	}
	var p domain.Profile
	if json.NewDecoder(r.Body).Decode(&p) != nil {
		http.Error(w, "bad profile", http.StatusBadRequest)
		return
	}
	if err := s.ProfileSave(p); err != nil {
		http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "children": len(p.Children)})
}
