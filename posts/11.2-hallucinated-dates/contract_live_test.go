//go:build live

// Live proof against the local model from post 00.2. Excluded from offline
// `go test` and GitHub CI (no model there). Run with: go test -tags live -v ./...
// Must fail if the model is down, never skip.
package main

import (
	"context"
	"testing"

	"github.com/t0ul/emaildrop/pkg/agent/ensemble"
	"github.com/t0ul/emaildrop/pkg/agent/extractor"
)

func TestModelHallucinatesDateReconcileDropsIt(t *testing.T) {
	ctx := context.Background()
	regexItems := extractor.ExtractEvents(email, "", year)

	// The model proposes a date the email never wrote, at least once.
	hallucinated := false
	for i := 0; i < 4 && !hallucinated; i++ {
		llm, err := llmExtract(ctx)
		if err != nil {
			t.Fatalf("model unreachable (bring up the lab from post 00.2): %v", err)
		}
		hallucinated = len(offText(llm)) > 0
	}
	if !hallucinated {
		t.Fatal("the undefended model should invent a date for the undated event; otherwise there is nothing to reconcile")
	}

	// Reconciled against the email's own dates, no invented date survives.
	for i := 0; i < 4; i++ {
		llm, err := llmExtract(ctx)
		if err != nil {
			t.Fatalf("model unreachable: %v", err)
		}
		out := ensemble.Reconcile(regexItems, llm, email, year)
		if bad := offText(out); len(bad) > 0 {
			t.Fatalf("a hallucinated date survived reconciliation on run %d: %v", i, bad)
		}
	}
}
