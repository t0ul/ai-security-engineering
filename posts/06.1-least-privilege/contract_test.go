package main

import "testing"

// Deterministic, no model. A naive runner writes a read-only tool's artifact; the
// real pipeline refuses it (capability enforcement) while still producing the write
// tool's .ics.

func TestNaiveWritesRogueArtifact(t *testing.T) {
	if !naiveWritesAnything() {
		t.Fatal("a naive runner should write the read-only tool's file; otherwise there is nothing to fix")
	}
}

func TestPipelineRefusesOverreach(t *testing.T) {
	if !pipelineEnforces() {
		t.Fatal("the pipeline must refuse the read-only tool's write and still produce the write tool's .ics")
	}
}
