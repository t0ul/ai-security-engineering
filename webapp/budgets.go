package webapp

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/t0ul/ai-security-engineering/controlplane"
)

// BudgetRow is one entry of the budgets tab (C9): a governed resource envelope.
// KeySet reports whether the env var named by the config's KeyRef is populated —
// so the console shows the key POINTER and whether it resolves, never the value.
type BudgetRow struct {
	Name    string                    `json:"name"`
	Version int                       `json:"version"`
	Hash    string                    `json:"hash"`
	Config  controlplane.BudgetConfig `json:"config"`
	KeySet  bool                      `json:"key_set"`
}

func (s *Server) budgetsList(w http.ResponseWriter, _ *http.Request) {
	var out []BudgetRow
	for _, v := range s.Budgets.List() {
		keySet := v.Config.KeyRef != "" && os.Getenv(v.Config.KeyRef) != ""
		out = append(out, BudgetRow{Name: v.Name, Version: v.Version, Hash: short12(v.Hash), Config: v.Config, KeySet: keySet})
	}
	writeJSON(w, map[string]any{"budgets": out})
}

func (s *Server) budgetsActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name   string                    `json:"name"`
		Config controlplane.BudgetConfig `json:"config"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		http.Error(w, "name and config required", http.StatusBadRequest)
		return
	}
	v := s.Budgets.Activate(req.Name, req.Config)
	writeJSON(w, map[string]any{"ok": true, "name": v.Name, "version": v.Version, "hash": short12(v.Hash)})
}

func (s *Server) budgetsReset(w http.ResponseWriter, r *http.Request) {
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
	v := s.Budgets.Reset(req.Name)
	writeJSON(w, map[string]any{"ok": true, "name": v.Name, "version": v.Version})
}

// overBudget enforces the governed per-minute request rate for the console API (a
// fixed-window limiter). RatePerMin <= 0 (or no Budgets) means unlimited. This is
// the one budget the app enforces itself; rate/token/spend on model calls are
// enforced gateway-side by gouncer.
func (s *Server) overBudget() bool {
	if s.Budgets == nil {
		return false
	}
	limit := s.Budgets.Config("api").RatePerMin
	if limit <= 0 {
		return false
	}
	s.rlMu.Lock()
	defer s.rlMu.Unlock()
	now := time.Now()
	if now.Sub(s.rlWindow) >= time.Minute {
		s.rlWindow = now
		s.rlCount = 0
	}
	s.rlCount++
	return s.rlCount > limit
}
