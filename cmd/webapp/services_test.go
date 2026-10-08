package main

import (
	"path/filepath"
	"testing"

	"github.com/t0ul/ai-security-engineering/pkg/datastore"
	"github.com/t0ul/ai-security-engineering/pkg/domain"
)

// These exercise the real typed services against a real SQLite datastore — no
// mocks. The point of the interface refactor is verified by driving the actual
// implementation through a live DB round-trip, which is what the app does.

func liveStore(t *testing.T) *datastore.Store {
	t.Helper()
	db, err := datastore.Open(filepath.Join(t.TempDir(), "inv.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestProfileStoreRoundTrip(t *testing.T) {
	ps := profileStore{inv: liveStore(t)}
	want := domain.Profile{Children: []domain.Child{
		{Name: "Ada", Grade: "K", HalfDays: []string{"2026-11-26"}},
	}}
	if err := ps.Save(want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := ps.Load() // reads it back out of real SQLite
	if len(got.Children) != 1 || got.Children[0].Name != "Ada" || got.Children[0].Grade != "K" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
	if len(got.Children[0].HalfDays) != 1 || got.Children[0].HalfDays[0] != "2026-11-26" {
		t.Fatalf("half-days not persisted: %+v", got.Children[0])
	}
}

func TestEvalServiceHistoryLive(t *testing.T) {
	db := liveStore(t)
	if err := db.RecordEval("samples/1.txt", 0.87); err != nil {
		t.Fatalf("record: %v", err)
	}
	es := &evalService{inv: db}
	h := es.History() // real ListEvals through the real service
	if len(h) != 1 || h[0].Label != "samples/1.txt" || h[0].F1 != 0.87 {
		t.Fatalf("history mismatch: %+v", h)
	}
	if h[0].At == "" {
		t.Error("expected a formatted timestamp from the DB row")
	}
}
