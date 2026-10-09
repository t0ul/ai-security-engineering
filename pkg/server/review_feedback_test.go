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

type recFlywheel struct{ accepts, rejects, other int }

func (f *recFlywheel) Record(decision, _, _ string) {
	switch decision {
	case "accept":
		f.accepts++
	case "reject":
		f.rejects++
	default:
		f.other++
	}
}
func (f *recFlywheel) Stats() (int, int)          { return f.accepts, f.rejects }
func (f *recFlywheel) Recent(int) []FeedbackEntry { return nil }

// TestReviewLinkageAndFeedback locks the Review CRUD wiring + feedback: a needs-review
// item is returned with its source file + matching Items index (so accept/edit work) and
// a key; and the feedback endpoint records a thumbs rating into the flywheel.
func TestReviewLinkageAndFeedback(t *testing.T) {
	outbox := t.TempDir()
	// One item, flagged for review (also present in needs_review).
	sidecar := `{"source":"nl.txt","tools":{},
	  "items":[{"title":"Picture Day","start":"2026-10-15","all_day":true,"kind":"event","confidence":0.55}],
	  "needs_review":[{"title":"Picture Day","start":"2026-10-15","all_day":true,"kind":"event","confidence":0.55}]}`
	if err := os.WriteFile(filepath.Join(outbox, "nl.summary.json"), []byte(sidecar), 0o644); err != nil {
		t.Fatal(err)
	}
	fw := &recFlywheel{}
	srv := httptest.NewServer(New(Config{OutboxDir: outbox, Flywheel: fw}).Handler())
	defer srv.Close()

	// Review returns the item with file + index + key.
	resp, err := http.Get(srv.URL + "/api/review")
	if err != nil {
		t.Fatal(err)
	}
	var d struct {
		Items []struct {
			Title string `json:"title"`
			File  string `json:"file"`
			Index int    `json:"index"`
			Key   string `json:"key"`
		} `json:"items"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&d)
	resp.Body.Close()
	if len(d.Items) != 1 {
		t.Fatalf("expected 1 review item, got %d", len(d.Items))
	}
	it := d.Items[0]
	if it.File != "nl.summary.json" || it.Index != 0 || it.Key == "" {
		t.Errorf("review item must carry file+index+key, got %+v", it)
	}

	// Feedback records into the flywheel.
	post := func(body string) int {
		r, err := http.Post(srv.URL+"/api/feedback", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		return r.StatusCode
	}
	post(`{"source":"review","label":"Picture Day","rating":"up"}`)
	post(`{"source":"review","label":"x","rating":"down"}`)
	post(`{"source":"chat","label":"y","rating":"neutral"}`)
	if fw.accepts != 1 || fw.rejects != 1 || fw.other != 1 {
		t.Errorf("flywheel must tally feedback: accepts=%d rejects=%d other=%d", fw.accepts, fw.rejects, fw.other)
	}
}
