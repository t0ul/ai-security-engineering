package webapp

import (
	"encoding/json"
	"net/http"
)

// Child is one kid's profile — the key that personalizes the daily timeline (R1).
// Household config set by the operator, never derived from an untrusted email.
type Child struct {
	Name    string `json:"name"`
	Grade   string `json:"grade,omitempty"`
	Class   string `json:"class,omitempty"`
	Teacher string `json:"teacher,omitempty"`
	School  string `json:"school,omitempty"`
	In      string `json:"in,omitempty"`    // arrival, e.g. "8:15"
	Lunch   string `json:"lunch,omitempty"` // lunch, e.g. "10:55"
	Out     string `json:"out,omitempty"`   // dismissal, e.g. "2:30"
	Notes   string `json:"notes,omitempty"` // allergies, bus, etc.

	HalfDays   []string `json:"half_days,omitempty"`    // ISO dates with early dismissal
	HalfDayOut string   `json:"half_day_out,omitempty"` // early dismissal time on a half day, e.g. "11:30"
}

// Profile is the household's children.
type Profile struct {
	Children []Child `json:"children"`
}

// loadProfile reads the household profile from the governed store (empty if none).
func (s *Server) loadProfile() Profile {
	if s.ProfileLoad != nil {
		return s.ProfileLoad()
	}
	return Profile{}
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
	var p Profile
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
