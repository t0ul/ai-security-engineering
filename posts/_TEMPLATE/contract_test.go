package main

import "testing"

// The lesson's contract as tests, so the gate and the folder agree with the post.
// Both must pass before a lesson is done, and `make board` re-runs them after
// every later lesson. If this lesson involves the model, these tests call the
// local model live (planner on 127.0.0.1:11435). They must fail if it is down,
// never skip: a test that skips is not testing anything.

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
