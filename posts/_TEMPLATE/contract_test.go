package main

import "testing"

// The lesson's contract as tests, so CI and the folder agree with the post.
// Both must pass before a lesson is done.

func TestAttackLandsUndefended(t *testing.T) {
	if !attackUndefended() {
		t.Fatal("the undefended path should let the attack land; otherwise there is nothing to fix")
	}
}

func TestAttackBlockedDefended(t *testing.T) {
	if !attackDefended() {
		t.Fatal("the defended path should block the attack; the control is not doing its job")
	}
}
