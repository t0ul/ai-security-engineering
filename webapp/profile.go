package webapp

import (
	"encoding/json"
	"net/http"
	"os"
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
}

// Profile is the household's children.
type Profile struct {
	Children []Child `json:"children"`
}

// loadProfile reads the profile JSON (empty profile if absent/unreadable).
func (s *Server) loadProfile() Profile {
	var p Profile
	if s.ProfilePath == "" {
		return p
	}
	raw, err := os.ReadFile(s.ProfilePath)
	if err != nil {
		return p
	}
	_ = json.Unmarshal(raw, &p)
	return p
}

func (s *Server) profileGet(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, s.loadProfile())
}

// profileSave writes the household profile (0600). CSRF + authz(write/profile)
// guard this route; the data stays on the host (no egress).
func (s *Server) profileSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	if s.ProfilePath == "" {
		http.Error(w, "no profile path configured", http.StatusNotFound)
		return
	}
	var p Profile
	if json.NewDecoder(r.Body).Decode(&p) != nil {
		http.Error(w, "bad profile", http.StatusBadRequest)
		return
	}
	raw, _ := json.MarshalIndent(p, "", "  ")
	if err := os.WriteFile(s.ProfilePath, raw, 0o600); err != nil {
		http.Error(w, "save failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "children": len(p.Children)})
}
