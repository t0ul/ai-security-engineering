package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/agent/ensemble"
	"github.com/t0ul/emaildrop/pkg/agent/schema"
)

// Offline, deterministic, no model: the reconciler's grounding property. Given a
// model proposal with one date in the email and one that is not, Reconcile keeps
// the grounded event and drops the off-text one. CI runs this; the model-in-the-
// loop proof (the model actually inventing the date) is in the live-tagged test.

func TestReconcileDropsOffTextDate(t *testing.T) {
	// The email names only October 2nd.
	llm := []schema.Event{
		{Title: "Picture Day", Start: "2026-10-02", AllDay: true}, // grounded
		{Title: "Winter Gala", Start: "2026-12-31", AllDay: true}, // invented
	}
	out := ensemble.Reconcile(nil, llm, email, year)

	if len(offText(out)) != 0 {
		t.Fatalf("reconcile must drop every off-text date, got %v", offText(out))
	}
	kept := false
	for _, e := range out {
		if e.Title == "Picture Day" {
			kept = true
		}
		if e.Title == "Winter Gala" {
			t.Fatal("the invented Winter Gala date must be dropped")
		}
	}
	if !kept {
		t.Fatal("the grounded Picture Day must survive reconciliation")
	}
}
