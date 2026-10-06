package webapp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/t0ul/ai-security-engineering/webapp"
	"github.com/t0ul/gledger"
)

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
