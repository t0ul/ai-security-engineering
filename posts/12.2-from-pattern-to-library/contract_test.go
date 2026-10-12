package main

import (
	"testing"

	"github.com/t0ul/ADD"
)

// Deterministic, no model. The suite runs through the real ADD gate: every
// declarative Technique passes its before/after invariants, and the suite yields
// an OWASP coverage grid. This is the per-lesson loop, extracted into the library.

func TestSuitePassesTheGate(t *testing.T) {
	outs := add.Gate(t, suite()...) // runs each row as a subtest, logs the grid
	if len(outs) != 3 {
		t.Fatalf("expected 3 evaluated techniques, got %d", len(outs))
	}
	for _, o := range outs {
		if o.UndefendedASR != 1.0 || o.DefendedASR != 0.0 {
			t.Fatalf("%s: want undefended 100%%/defended 0%%, got %.0f/%.0f", o.Name, o.UndefendedASR*100, o.DefendedASR*100)
		}
	}
}

func TestSuiteCoversThreeRisks(t *testing.T) {
	outs := make([]add.Outcome, 0)
	for _, tech := range suite() {
		o, v := add.Evaluate(tech)
		if len(v) != 0 {
			t.Fatalf("%s should pass, got %v", tech.Name, v)
		}
		outs = append(outs, o)
	}
	cov := add.Coverage(outs)
	for _, risk := range []string{"LLM01", "LLM02", "LLM07"} {
		if len(cov[risk]) == 0 {
			t.Fatalf("coverage grid missing risk %s", risk)
		}
	}
}
