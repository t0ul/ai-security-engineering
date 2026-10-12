package main

import (
	"strings"
	"testing"
)

// Deterministic, no model (a dead port stands in for "model down"). The two
// dishonest gates report green while testing nothing; the honest gate fails
// closed when its target is unreachable. The live number is in the live-tagged
// test, which needs the real model.

func TestStubProvesNothing(t *testing.T) {
	// A stub that always refuses can never be beaten, so its row is green with or
	// without a real defense: it is not a test.
	if strings.Contains(stubModel("anything"), marker) {
		t.Fatal("the stub should never return the marker; that is exactly why it proves nothing")
	}
}

func TestNaiveGateSkipsToGreen(t *testing.T) {
	pass, note := naiveGateSkips(deadBase)
	if !pass || !strings.Contains(note, "skipped") {
		t.Fatalf("the naive gate should report a (false) pass by skipping when the model is down, got pass=%v note=%q", pass, note)
	}
}

func TestHonestGateFailsClosed(t *testing.T) {
	pass, err := honestGate(deadBase)
	if pass || err == nil {
		t.Fatalf("the honest gate must FAIL (not skip) when the model is unreachable, got pass=%v err=%v", pass, err)
	}
	if !strings.Contains(err.Error(), "does not skip") {
		t.Fatalf("the failure should say the gate does not skip, got %v", err)
	}
}
