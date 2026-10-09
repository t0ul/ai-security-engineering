package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/t0ul/gledger"
)

// FrontierEndpoint points a subject (an NHI role) at an off-host frontier model: the
// inference endpoint and a KeyRef — the NAME of the env var that holds the API token,
// never the token value (secret-by-pointer, C9 convention). Binding a subject here marks
// it frontier-bound, so the residency policy refuses it any confidential list/export
// (data must not ship off-host). KeySet reports whether the named env var resolves.
type FrontierEndpoint struct {
	Subject  string `json:"subject"`
	Endpoint string `json:"endpoint"`
	KeyRef   string `json:"key_ref"`
	KeySet   bool   `json:"key_set"`
}

// FrontierStore persists the frontier endpoint bindings (secret-by-pointer). List
// resolves KeySet; it never returns the token value. Nil = no Frontier card.
type FrontierStore interface {
	ListFrontier() ([]FrontierEndpoint, error)
	UpsertFrontier(subject, endpoint, keyRef string) error
	DeleteFrontier(subject string) error
}

func (s *Server) frontierList(w http.ResponseWriter, _ *http.Request) {
	rows, err := s.Frontier.ListFrontier()
	if err != nil {
		http.Error(w, "frontier read failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"frontier": rows})
}

func (s *Server) frontierUpsert(w http.ResponseWriter, r *http.Request) {
	var req FrontierEndpoint
	if json.NewDecoder(r.Body).Decode(&req) != nil || strings.TrimSpace(req.Subject) == "" || strings.TrimSpace(req.Endpoint) == "" {
		http.Error(w, "subject and endpoint are required", http.StatusBadRequest)
		return
	}
	// Reject a literal token pasted into key_ref: it must be an env-var NAME, not a
	// secret. A real token has characters an env-var name never does.
	if strings.ContainsAny(req.KeyRef, " \t:/\"'") {
		http.Error(w, "key_ref must be an env var NAME (a pointer), never the token value", http.StatusBadRequest)
		return
	}
	if err := s.Frontier.UpsertFrontier(req.Subject, req.Endpoint, req.KeyRef); err != nil {
		http.Error(w, "frontier write failed", http.StatusInternalServerError)
		return
	}
	if s.Audit != nil {
		s.Audit.Emit(gledger.NewTraceID(), "frontier", "upsert", gledger.F{"subject": req.Subject, "endpoint": req.Endpoint, "key_ref": req.KeyRef})
	}
	writeJSON(w, map[string]any{"ok": true, "subject": req.Subject})
}

func (s *Server) frontierDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Subject string `json:"subject"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Subject == "" {
		http.Error(w, "subject required", http.StatusBadRequest)
		return
	}
	if err := s.Frontier.DeleteFrontier(req.Subject); err != nil {
		http.Error(w, "frontier delete failed", http.StatusInternalServerError)
		return
	}
	if s.Audit != nil {
		s.Audit.Emit(gledger.NewTraceID(), "frontier", "delete", gledger.F{"subject": req.Subject})
	}
	writeJSON(w, map[string]any{"ok": true, "subject": req.Subject})
}
