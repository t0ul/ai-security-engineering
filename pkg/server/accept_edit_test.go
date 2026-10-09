package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAcceptWithEdits locks the edit-before-accept feature: an operator's edits (title,
// time, location) are applied to the extracted event and written to the .ics — the human
// touch — through the same two-phase HITL confirm. Real Server + real sidecar + real .ics.
func TestAcceptWithEdits(t *testing.T) {
	outbox := t.TempDir()
	sidecar := `{"source":"nl.txt","tools":{},"items":[{"title":"back to school","start":"2026-09-29","all_day":true,"kind":"event","confidence":0.6}]}`
	if err := os.WriteFile(filepath.Join(outbox, "nl.summary.json"), []byte(sidecar), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(New(Config{OutboxDir: outbox}).Handler())
	defer srv.Close()

	post := func(body string) map[string]any {
		resp, err := http.Post(srv.URL+"/api/accept", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&m)
		return m
	}

	edit := `"edit":{"title":"Back to School Night","start":"2026-09-29T18:30:00","all_day":false,"location":"Auditorium"}`
	// Phase 1: challenge — the evidence must reflect the EDITED values.
	ch := post(`{"file":"nl.summary.json","index":0,` + edit + `}`)
	nonce, _ := ch["nonce"].(string)
	if nonce == "" {
		t.Fatalf("expected a HITL nonce, got %v", ch)
	}
	if sum, _ := ch["summary"].(string); !strings.Contains(sum, "EDITED") || !strings.Contains(sum, "Back to School Night") {
		t.Errorf("challenge summary should reflect the edit, got %q", sum)
	}
	// Phase 2: confirm → the edited event is written.
	res := post(`{"file":"nl.summary.json","index":0,` + edit + `,"nonce":"` + nonce + `","confirm":true}`)
	out, _ := res["file"].(string)
	if ok, _ := res["ok"].(bool); !ok || out == "" {
		t.Fatalf("accept should write an .ics, got %v", res)
	}

	ics, err := os.ReadFile(filepath.Join(outbox, out))
	if err != nil {
		t.Fatal(err)
	}
	body := string(ics)
	if !strings.Contains(body, "Back to School Night") {
		t.Errorf("written .ics must carry the edited title, got:\n%s", body)
	}
	if !strings.Contains(body, "Auditorium") {
		t.Errorf("written .ics must carry the edited location, got:\n%s", body)
	}
	// The edited start carries a time (not all-day), so the .ics has a timed DTSTART.
	if !strings.Contains(body, "T183000") && !strings.Contains(body, "T18:30") {
		t.Errorf("written .ics must carry the edited time, got:\n%s", body)
	}
}
