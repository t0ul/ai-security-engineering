package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/t0ul/gledger"
)

// Deterministic, no model. One poisoned email lands all three payloads on a naive
// agent; the real end-to-end pipeline contains all three, and the trace replayed
// by its id shows each layer that held, over a verified audit chain. The secret
// and the exfil URL are benign markers.

func TestNaivePayloadsAllLand(t *testing.T) {
	s, i, a := naiveLands()
	if !s || !i || !a {
		t.Fatalf("every payload must land on the naive agent (secret=%v injection=%v artifact=%v); otherwise there is nothing to contain", s, i, a)
	}
}

func TestPipelineContainsEverything(t *testing.T) {
	c := pipelineContains()
	defer os.RemoveAll(filepath.Dir(c.logPath))
	if c.secretIndexed {
		t.Error("the AWS key must be scrubbed before indexing")
	}
	if c.injectionReached {
		t.Error("the injection must be neutralized before the extractor sees it")
	}
	if c.artifactWritten {
		t.Error("the over-capable tool's .ics must be refused")
	}
}

func TestTraceReplayShowsEveryLayer(t *testing.T) {
	c := pipelineContains()
	defer os.RemoveAll(filepath.Dir(c.logPath))

	recs, err := gledger.Read(c.logPath)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"pii/scrubbed":                         false,
		"injection_guard/detected":             false,
		"event_extractor/capability_violation": false,
	}
	for _, r := range recs {
		if r.TraceID != c.trace {
			continue
		}
		want[r.Span+"/"+r.Event] = true
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("trace is missing the layer event %q", k)
		}
	}
	if ok, _ := gledger.VerifyFile(c.logPath); !ok {
		t.Error("the audit chain must verify (tamper-evident replay)")
	}
}
