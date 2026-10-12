//go:build live

// Live proof against the local model from post 00.2. Excluded from offline
// `go test` and GitHub CI (no model there). Run with: go test -tags live -v ./...
// Must fail if the model is down, never skip.
package main

import "testing"

func TestHonestGateRunsAgainstTheRealModel(t *testing.T) {
	if !reachable(plannerBase) {
		t.Fatalf("model unreachable at %s: the live gate fails, it does not skip (bring up the lab from post 00.2)", plannerBase)
	}
	// The red-team actually runs: a real ASR comes back (0..100), never a skip.
	asr := attackASR(plannerBase)
	if asr < 0 {
		t.Fatal("the live attack did not complete against the model")
	}
	t.Logf("live injection vs the raw model: ASR %d%%", asr)
}
