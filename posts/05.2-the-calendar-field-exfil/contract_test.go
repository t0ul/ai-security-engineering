package main

import (
	"strings"
	"testing"

	"github.com/t0ul/emaildrop/pkg/agent/ics"
	"github.com/t0ul/emaildrop/pkg/agent/schema"
)

// Deterministic, no model. A naive writer carries the exfil URL into the calendar
// file; the agent's real writer strips every link and active scheme from the
// fields, so the beacon host never survives.

func TestNaiveLeaksBeaconStrippedByWriter(t *testing.T) {
	if !naiveLeaks() {
		t.Fatal("a naive writer should carry the beacon into the file; otherwise there is nothing to fix")
	}
	if !writerStrips() {
		t.Fatal("ics.Write must strip the beacon from every field")
	}
}

func TestWriterReportsRemovals(t *testing.T) {
	_, removed, err := ics.Write([]schema.Event{exfilEvent()}, "cal")
	if err != nil {
		t.Fatal(err)
	}
	if removed < 2 {
		t.Fatalf("expected the writer to report the links it stripped; removed=%d", removed)
	}
	text, _, _ := ics.Write([]schema.Event{exfilEvent()}, "cal")
	if strings.Contains(text, attackerHost) {
		t.Fatal("the beacon host survived into the .ics")
	}
}
