// Package webapp is the local operator web app for managing the agent: run the
// security scorecard from the UI, and browse + replay incidents from the
// chain-verified audit log. It reuses the same Go components the agent runs on
// (redteam, ir), so the console and the system under test are one binary.
package webapp

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/t0ul/ai-security-engineering/ir"
	"github.com/t0ul/ai-security-engineering/redteam"
	"github.com/t0ul/gorauder"
)

// Server serves the dashboard and its API.
type Server struct {
	AuditPath string // gledger log to read incidents from
	OutboxDir string // accepted .ics artifacts to render + offer for download
	InboxPath string // where the user drops .txt emails (shown in the UI)
}

// Handler returns the app routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/scorecard", s.scorecard)
	mux.HandleFunc("/api/incidents", s.incidents)
	mux.HandleFunc("/api/incident", s.incident)
	mux.HandleFunc("/api/events", s.events)
	if s.InboxPath != "" {
		mux.HandleFunc("/api/drop", s.drop)
	}
	if s.OutboxDir != "" {
		mux.Handle("/ics/", http.StripPrefix("/ics/", http.FileServer(http.Dir(s.OutboxDir))))
	}
	mux.HandleFunc("/", s.index)
	return mux
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

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
