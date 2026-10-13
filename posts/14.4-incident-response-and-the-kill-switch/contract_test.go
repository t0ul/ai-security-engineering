package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
)

// Deterministic, no model. An ungoverned switch can be flipped by anyone and only
// halts fully (a deny storm); the governed switch refuses an unauthorized toggle,
// and its layered block-tools level stops side effects while reads keep serving.

func TestUngovernedSwitchIsFlippableAndStorms(t *testing.T) {
	if !ungovernedUndefended() {
		t.Fatal("an ungoverned switch must be flippable by anyone and only stop by full halt; otherwise there is nothing to govern")
	}
}

func TestGovernedLayeredSwitch(t *testing.T) {
	if !governedLayeredDefended() {
		t.Fatal("the governed switch must refuse an unauthorized toggle, block tools while allowing reads, and escalate to a full halt")
	}
}

func TestBlockToolsAllowsReadsStopsTools(t *testing.T) {
	s := controlplane.NewSafety(nil)
	s.Set("sre", controlplane.LevelBlockTools)
	if s.AllowToolExec() {
		t.Fatal("block-tools must stop tool/side-effect execution")
	}
	if !s.AllowRequest() {
		t.Fatal("block-tools must still allow a request/read (no deny storm)")
	}
}
