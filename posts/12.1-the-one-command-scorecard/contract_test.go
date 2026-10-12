package main

import (
	"strings"
	"testing"
)

// Deterministic, no model. The real ADD gate passes a wired row (reachable RED,
// control holds, control on the path) and flags the two false-green rows: a
// stubbed undefended arm ("proves nothing") and a control that exists but is not
// wired into the real path.

func TestWiredRowPasses(t *testing.T) {
	o, v := report(wiredRow())
	if len(v) != 0 {
		t.Fatalf("a wired row must pass the gate, got violations: %v", v)
	}
	if o.UndefendedASR != 1.0 || o.DefendedASR != 0.0 || o.WiredASR != 0.0 {
		t.Fatalf("wired row ASRs want 100/0/0, got %.0f/%.0f/%.0f", o.UndefendedASR*100, o.DefendedASR*100, o.WiredASR*100)
	}
}

func TestStubbedRowIsFlagged(t *testing.T) {
	_, v := report(stubbedRow())
	if !anyContains(v, "NOT PROVEN") {
		t.Fatalf("a stubbed undefended arm must be flagged as proving nothing, got %v", v)
	}
}

func TestNotWiredRowIsFlagged(t *testing.T) {
	_, v := report(notWiredRow())
	if !anyContains(v, "NOT WIRED") {
		t.Fatalf("a control that is not on the real path must be flagged NOT WIRED, got %v", v)
	}
}

func anyContains(xs []string, sub string) bool {
	for _, x := range xs {
		if strings.Contains(x, sub) {
			return true
		}
	}
	return false
}
