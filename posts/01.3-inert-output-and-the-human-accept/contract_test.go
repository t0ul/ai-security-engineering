package main

import "testing"

// The lesson's contract as tests, so the gate and the folder agree with the post.
// This lesson is deterministic: no model decides whether an event reaches the
// calendar. The accept gate does, and these assert it.

func TestAttackLandsUndefended(t *testing.T) {
	if !attackUndefended() {
		t.Fatal("auto-add should put the flagged event on the calendar; otherwise there is nothing to fix")
	}
}

func TestAttackBlockedDefended(t *testing.T) {
	if !attackDefended() {
		t.Fatal("the inert-file-plus-accept path should keep the flagged event off the calendar")
	}
}

func TestCleanEventAccepted(t *testing.T) {
	if !cleanEventAccepted() {
		t.Fatal("a human accepting the safe default must still add the clean event")
	}
}
