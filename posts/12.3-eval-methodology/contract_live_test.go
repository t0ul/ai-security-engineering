//go:build live

// Live proof against the local model from post 00.2. Excluded from offline
// `go test` and GitHub CI (no model there). Run with: go test -tags live -v ./...
// Must fail if the model is down, never skip.
package main

import (
	"context"
	"testing"
)

func TestNaiveJudgeGamedHardenedHolds(t *testing.T) {
	ctx := context.Background()

	// The embedded directive flips the naive judge at least once.
	gamed := false
	for i := 0; i < 4 && !gamed; i++ {
		out, err := chat(ctx, naiveJudgeSystem(), naiveJudgeUser())
		if err != nil {
			t.Fatalf("model unreachable (bring up the lab from post 00.2): %v", err)
		}
		gamed = score(out) >= gamedThreshold
	}
	if !gamed {
		t.Fatal("the naive judge should be gamed by the embedded directive; otherwise there is nothing to defend")
	}

	// The hardened judge grades the wrong answer low every time.
	for i := 0; i < 4; i++ {
		out, err := chat(ctx, hardenedJudgeSystem, hardenedJudgeUser())
		if err != nil {
			t.Fatalf("model unreachable: %v", err)
		}
		if s := score(out); s >= gamedThreshold {
			t.Fatalf("the hardened judge was gamed on run %d: score %d", i, s)
		}
	}
}
