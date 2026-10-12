package main

import "testing"

// Deterministic, no model. One brain executes a command hidden in the email; two
// brains, a data-only reader and an action-only planner, cannot, because the
// reader's output has no channel that can become a control action.

func TestOneBrainRunsInjectedCommand(t *testing.T) {
	if !oneBrainActsOnTheEmail() {
		t.Fatal("a single brain should run the email's injected command; otherwise there is nothing to fix")
	}
}

func TestTwoBrainsKeepDataNonExecutable(t *testing.T) {
	if !twoBrainsKeepDataFromBecomingControl() {
		t.Fatal("the planner must act only on its goal, never on a command carried in the reader's data")
	}
}
