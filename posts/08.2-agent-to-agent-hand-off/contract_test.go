package main

import "testing"

// Deterministic, no model. Every a2a-spoof lands on the naive executor; the
// authenticated executor refuses all of them while a genuine plan still runs and
// a captured hand-off cannot be replayed. The attack carries a benign marker
// command (attackCmd), recorded and never executed.

func TestSpoofLandsUndefended(t *testing.T) {
	if !spoofLandsUndefended() {
		t.Fatal("every a2a-spoof must land on the naive hand-off; otherwise there is nothing to defend")
	}
}

func TestSpoofRefusedDefended(t *testing.T) {
	if !gateRefusesSpoofs() {
		t.Fatal("the authenticated executor must refuse every spoof (ASR 0%)")
	}
}

func TestLegitHandoffExecutes(t *testing.T) {
	if !legitHandoffExecutes() {
		t.Fatal("the genuine planner's signed plan must still execute")
	}
}

func TestReplayRefused(t *testing.T) {
	if !replayRefused() {
		t.Fatal("a captured genuine hand-off must be refused on replay")
	}
}
