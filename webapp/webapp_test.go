package webapp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/t0ul/ai-security-engineering/agent/ics"
	"github.com/t0ul/ai-security-engineering/agent/schema"
	"github.com/t0ul/ai-security-engineering/webapp"
	"github.com/t0ul/gledger"
)

func TestEventsAPIReadsICS(t *testing.T) {
	dir := t.TempDir()
	icsText, _, _ := ics.Write([]schema.Event{schema.New("PTA Meeting", "2026-09-24T08:30:00")}, "cal")
	os.WriteFile(filepath.Join(dir, "3.events.ics"), []byte(icsText), 0o644)

	srv := httptest.NewServer((&webapp.Server{OutboxDir: dir}).Handler())
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/api/events")
	defer resp.Body.Close()
	var out struct {
		Events []webapp.Event `json:"events"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Events) != 1 || out.Events[0].Title != "PTA Meeting" || !out.Events[0].HasReminder {
		t.Fatalf("expected 1 PTA event with reminder, got %+v", out.Events)
	}
}

func TestScorecardAPIAllPass(t *testing.T) {
	srv := httptest.NewServer((&webapp.Server{}).Handler())
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/scorecard")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Results []map[string]any `json:"results"`
		AllPass bool             `json:"all_pass"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Results) == 0 || !out.AllPass {
		t.Fatalf("scorecard should run and pass: results=%d all_pass=%v", len(out.Results), out.AllPass)
	}
}

func TestDropWritesToInbox(t *testing.T) {
	inbox := t.TempDir()
	srv := httptest.NewServer((&webapp.Server{InboxPath: inbox}).Handler())
	defer srv.Close()
	resp, err := http.PostForm(srv.URL+"/api/drop", map[string][]string{"text": {"Back to School Night Sept 29th 6PM"}})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("drop returned %d", resp.StatusCode)
	}
	entries, _ := os.ReadDir(inbox)
	if len(entries) != 1 {
		t.Fatalf("expected one dropped file in inbox, got %d", len(entries))
	}
}

func TestIncidentsAPI(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	a, err := gledger.Open(path, "test")
	if err != nil {
		t.Fatal(err)
	}
	trace := gledger.NewTraceID()
	a.Emit(trace, "request", "start", nil)
	a.Emit(trace, "policy", "gate", gledger.F{"decision": "block"})

	srv := httptest.NewServer((&webapp.Server{AuditPath: path}).Handler())
	defer srv.Close()
	resp, _ := http.Get(srv.URL + "/api/incidents")
	defer resp.Body.Close()
	var out struct {
		Traces []struct {
			ID string `json:"id"`
			N  int    `json:"n"`
		} `json:"traces"`
	}
	json.NewDecoder(resp.Body).Decode(&out)
	if len(out.Traces) != 1 || out.Traces[0].N != 2 {
		t.Fatalf("expected one 2-event incident, got %+v", out.Traces)
	}
}
