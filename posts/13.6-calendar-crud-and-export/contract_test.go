package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/durable"
)

// Deterministic, no model. Re-ingesting one email writes the event twice on a
// naive write, and exactly once through the durable ledger, which recognizes the
// replay by the event's content key. No attacker; the hazard is a safe retry.

func TestReingestDuplicatesUndefended(t *testing.T) {
	if !duplicatesUndefended() {
		t.Fatal("a naive write must duplicate on re-ingest; otherwise there is nothing to make idempotent")
	}
}

func TestDurableLedgerWritesExactlyOnce(t *testing.T) {
	if !exactlyOnceDefended() {
		t.Fatal("the durable ledger must write the event exactly once and recognize the replay")
	}
}

func TestReplayReturnsCachedResult(t *testing.T) {
	resetCalendar()
	l, _ := durable.Open("")
	e := events()[0]
	out1, replayed1, _ := l.Do("import", "calendar-write", eventKey(e), func() (string, error) { return addEvent(e.Title) })
	out2, replayed2, _ := l.Do("import", "calendar-write", eventKey(e), func() (string, error) { return addEvent(e.Title) })
	if replayed1 || !replayed2 {
		t.Fatalf("first call should run (replayed=false), second should be a replay (replayed=true); got %v, %v", replayed1, replayed2)
	}
	if out1 != out2 {
		t.Fatalf("the replay must return the first call's cached result, got %q vs %q", out1, out2)
	}
	if len(calendar) != 1 {
		t.Fatalf("the side effect must fire exactly once, calendar has %d", len(calendar))
	}
}
