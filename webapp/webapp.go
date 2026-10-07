// Package webapp is the local operator web app for managing the agent: run the
// security scorecard from the UI, and browse + replay incidents from the
// chain-verified audit log. It reuses the same Go components the agent runs on
// (redteam, ir), so the console and the system under test are one binary.
package webapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/t0ul/ai-security-engineering/controlplane"
	"github.com/t0ul/ai-security-engineering/ir"
	"github.com/t0ul/ai-security-engineering/netpolicy"
	"github.com/t0ul/ai-security-engineering/provenance"
	"github.com/t0ul/ai-security-engineering/redteam"
	"github.com/t0ul/gorauder"
)

// Server serves the dashboard and its API.
type Server struct {
	AuditPath string // gledger log to read incidents from
	OutboxDir string // accepted .ics artifacts to render + offer for download
	InboxPath string // where the user drops .txt emails (shown in the UI)

	// Egress is the action-link allowlist (deny-by-default). Fetch performs the
	// vetted fetch — in production controlplane.SandboxFetchTool (in the MicroVM).
	// Both unset = action links are refused.
	Egress netpolicy.Policy
	Fetch  Fetcher
	// Verifier, when set, checks each .ics against its .sig content credential so
	// the UI can show whether the agent provably produced it unaltered (M20).
	Verifier *provenance.Verifier
	// Safety, when set, is the layered kill switch (M18): Pause refuses new
	// processing/drops; BlockTools refuses accept/action side effects.
	Safety *controlplane.Safety
	// Search, when set, answers Ask-School queries over the (PII-scrubbed)
	// retrieval corpus. Nil = the Ask tab returns nothing.
	Search func(query string, k int) ([]SearchHit, error)

	// pending holds issued-but-unconfirmed HITL approvals, keyed by nonce (ASI09:
	// evidence-first, single-use, clickjack/forgery-resistant confirm).
	mu   sync.Mutex
	pend map[string]pending
}

// verifySig reports whether <name>.sig is a valid content credential over the
// given .ics bytes from a trusted agent key.
func (s *Server) verifySig(name string, content []byte) bool {
	if s.Verifier == nil {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(s.OutboxDir, name+".sig"))
	if err != nil {
		return false
	}
	var m provenance.Mark
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	return s.Verifier.Verify(content, m) == nil
}

// Handler returns the app routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/scorecard", s.scorecard)
	mux.HandleFunc("/api/incidents", s.incidents)
	mux.HandleFunc("/api/incident", s.incident)
	mux.HandleFunc("/api/events", s.events)
	mux.HandleFunc("/api/items", s.items)
	mux.HandleFunc("/api/summary", s.summary)
	mux.HandleFunc("/api/review", s.review)
	mux.HandleFunc("/api/activity", s.activity)
	mux.HandleFunc("/api/safety", s.safetyState)
	mux.HandleFunc("/api/killswitch", csrf(s.killswitch))
	mux.HandleFunc("/api/ask", s.ask)
	if s.InboxPath != "" {
		mux.HandleFunc("/api/drop", csrf(s.drop))
	}
	if s.OutboxDir != "" {
		mux.HandleFunc("/api/accept", csrf(s.accept))
		mux.HandleFunc("/api/action", csrf(s.action))
		mux.HandleFunc("/ics/", s.serveICS) // .ics only — NOT the whole outbox (sidecars hold PII)
	}
	mux.HandleFunc("/", s.index)
	return mux
}

// serveICS serves ONLY .ics artifacts from the outbox by bare basename. The
// outbox also holds <stem>.summary.json sidecars (contacts/digest/PII), so a
// blanket file server would leak them.
func (s *Server) serveICS(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/ics/")
	if name == "" || filepath.Base(name) != name || !strings.HasSuffix(strings.ToLower(name), ".ics") {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.OutboxDir, name))
}

// csrf rejects cross-site POSTs. The app binds loopback, but that does NOT stop
// CSRF — any site the browser visits can POST to 127.0.0.1. A foreign Origin is
// refused; same-origin fetches (loopback Origin, or no Origin) pass.
func csrf(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" {
			u, err := url.Parse(o)
			if err != nil || !isLoopbackHost(u.Hostname()) {
				http.Error(w, "cross-origin request refused", http.StatusForbidden)
				return
			}
		}
		h(w, r)
	}
}

func isLoopbackHost(h string) bool {
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

type scoreRow struct {
	Name      string  `json:"name"`
	Technique string  `json:"technique"`
	Before    float64 `json:"before"`
	After     float64 `json:"after"`
	Pass      bool    `json:"pass"`
}

func asr(target gorauder.Target, seeds []gorauder.Seed) float64 {
	return gorauder.NewRunner(target, gorauder.WithScorer(redteam.Scorer())).Run(context.Background(), seeds).ASR()
}

func (s *Server) scorecard(w http.ResponseWriter, _ *http.Request) {
	var rows []scoreRow
	allPass := true
	for _, c := range redteam.Cases() {
		before := asr(c.Undefended, c.Seeds) * 100
		after := asr(c.Defended, c.Seeds) * 100
		pass := after == 0
		if !pass {
			allPass = false
		}
		rows = append(rows, scoreRow{c.Name, c.Technique, before, after, pass})
	}
	writeJSON(w, map[string]any{"results": rows, "all_pass": allPass})
}

func (s *Server) incidents(w http.ResponseWriter, _ *http.Request) {
	events, _ := ir.Load(s.AuditPath)
	type row struct {
		ID string `json:"id"`
		N  int    `json:"n"`
	}
	var out []row
	for _, id := range ir.Traces(events) {
		out = append(out, row{id, len(ir.Timeline(events, id))})
	}
	writeJSON(w, map[string]any{"traces": out})
}

func (s *Server) incident(w http.ResponseWriter, r *http.Request) {
	trace := r.URL.Query().Get("trace")
	events, _ := ir.Load(s.AuditPath)
	writeJSON(w, map[string]any{"timeline": ir.Timeline(events, trace)})
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

// safetyState reports the current kill-switch level.
func (s *Server) safetyState(w http.ResponseWriter, _ *http.Request) {
	lvl := "none"
	allowReq, allowTools := true, true
	if s.Safety != nil {
		lvl = s.Safety.Level().String()
		allowReq, allowTools = s.Safety.AllowRequest(), s.Safety.AllowToolExec()
	}
	writeJSON(w, map[string]any{"level": lvl, "allow_request": allowReq, "allow_tools": allowTools})
}

// killswitch sets the kill level (0 none, 1 block-tools, 2 pause, 3 halt).
func (s *Server) killswitch(w http.ResponseWriter, r *http.Request) {
	if s.Safety == nil {
		http.Error(w, "no kill switch configured", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Level int `json:"level"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Level < 0 || req.Level > 3 {
		http.Error(w, "bad level", http.StatusBadRequest)
		return
	}
	s.Safety.Set("operator", controlplane.KillLevel(req.Level))
	writeJSON(w, map[string]any{"ok": true, "level": s.Safety.Level().String()})
}

// paused reports whether new processing/drops are refused.
func (s *Server) paused() bool { return s.Safety != nil && !s.Safety.AllowRequest() }

// toolsBlocked reports whether side-effect actions (accept/fetch) are refused.
func (s *Server) toolsBlocked() bool { return s.Safety != nil && !s.Safety.AllowToolExec() }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
