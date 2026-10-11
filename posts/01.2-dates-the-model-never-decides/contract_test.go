package main

import "testing"

// The lesson's contract as tests, so the gate and the folder agree with the post.
// Both must pass before a lesson is done, and `make board` re-runs them after
// every later lesson. This lesson does not involve the model: the dates are
// decided by deterministic code on purpose, which is the whole point.

func TestAttackLandsUndefended(t *testing.T) {
	if !attackUndefended() {
		t.Fatal("the undefended path should calendar both wrong dates with no warning; otherwise there is nothing to fix")
	}
}

func TestAttackBlockedDefended(t *testing.T) {
	if !attackDefended() {
		t.Fatal("the defended path should hold the weekday mismatch for review and reject the impossible date")
	}
}

func TestCleanDateStillCalendared(t *testing.T) {
	if !cleanStillCalendared() {
		t.Fatal("the control must still calendar a correct date, or it is just refusing everything")
	}
}
