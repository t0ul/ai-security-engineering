package controlplane

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/t0ul/goverlord"
)

// ConsoleServer exposes the governed control plane (Governance) over HTTP: the
// live, wired-to-real-state backbone the operator console (gridge GUI) sits on.
// Every mutation flows through goverlord, so RBAC, four-eyes, the kill switch,
// and the audit trail are enforced server-side, not in the UI.
type ConsoleServer struct {
	Gov *Governance
}

// Handler returns the console's routes: a JSON API under /api and the operator
// page at /.
func (s *ConsoleServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/state", s.state)
	mux.HandleFunc("/api/propose", s.propose)
	mux.HandleFunc("/api/approve", s.approve)
	mux.HandleFunc("/api/reject", s.reject)
	mux.HandleFunc("/api/rollback", s.rollback)
	mux.HandleFunc("/api/killswitch", s.killswitch)
	mux.HandleFunc("/api/history", s.history)
	mux.HandleFunc("/", s.index)
	return mux
}

// StateView is the operator's read model of the governed plane.
type StateView struct {
	Version int                  `json:"version"`
	Killed  bool                 `json:"killed"`
	Config  map[string]any       `json:"config"`
	History []goverlord.Version  `json:"history"`
	Pending []goverlord.Proposal `json:"pending"`
}

func (s *ConsoleServer) state(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, StateView{
		Version: s.Gov.Version(),
		Killed:  s.Gov.Killed(),
		Config:  s.Gov.Config(),
		History: s.Gov.History(),
		Pending: s.Gov.Pending(),
	})
}

func (s *ConsoleServer) propose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Op   string         `json:"op"`
		Note string         `json:"note"`
		Set  map[string]any `json:"set"`
	}
	if !decode(w, r, &req) {
		return
	}
	id, applied, err := s.Gov.ProposeConfig(req.Op, req.Note, req.Set)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"proposal_id": id, "applied": applied})
}

func (s *ConsoleServer) approve(w http.ResponseWriter, r *http.Request) {
	var req struct{ Op, ID string }
	if !decodeOpID(w, r, &req.Op, &req.ID) {
		return
	}
	applied, err := s.Gov.Approve(req.Op, req.ID)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": applied})
}

func (s *ConsoleServer) reject(w http.ResponseWriter, r *http.Request) {
	var op, id string
	if !decodeOpID(w, r, &op, &id) {
		return
	}
	if err := s.Gov.Reject(op, id); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *ConsoleServer) rollback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Op string `json:"op"`
		To int    `json:"to"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.Gov.Rollback(req.Op, req.To); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": s.Gov.Version()})
}

func (s *ConsoleServer) killswitch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Op     string `json:"op"`
		Engage bool   `json:"engage"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.Gov.SetKillSwitch(req.Op, req.Engage); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"killed": s.Gov.Killed()})
}

// history returns the durable decision log from the inventory (empty when no
// inventory is attached).
func (s *ConsoleServer) history(w http.ResponseWriter, r *http.Request) {
	if s.Gov.Inventory == nil {
		writeJSON(w, http.StatusOK, map[string]any{"approvals": []any{}})
		return
	}
	ap, err := s.Gov.Inventory.ListApprovals(50)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"approvals": ap})
}

func (s *ConsoleServer) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(consoleHTML))
}

// --- helpers ---

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return false
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad JSON: " + err.Error()})
		return false
	}
	return true
}

func decodeOpID(w http.ResponseWriter, r *http.Request, op, id *string) bool {
	var req struct{ Op, ID string }
	if !decode(w, r, &req) {
		return false
	}
	*op, *id = req.Op, req.ID
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	writeJSON(w, statusFor(err), map[string]string{"error": err.Error()})
}

// statusFor maps goverlord's typed errors to HTTP status codes.
func statusFor(err error) int {
	switch {
	case errors.Is(err, goverlord.ErrForbidden):
		return http.StatusForbidden // 403
	case errors.Is(err, goverlord.ErrKilled):
		return http.StatusConflict // 409
	case errors.Is(err, goverlord.ErrSameOperator):
		return http.StatusUnprocessableEntity // 422
	case errors.Is(err, goverlord.ErrNoProposal):
		return http.StatusNotFound // 404
	case errors.Is(err, goverlord.ErrUnknownOperator):
		return http.StatusUnauthorized // 401
	case errors.Is(err, goverlord.ErrBadVersion):
		return http.StatusBadRequest // 400
	default:
		return http.StatusInternalServerError
	}
}
