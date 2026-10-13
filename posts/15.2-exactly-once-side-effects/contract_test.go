package main

import (
	"errors"
	"testing"

	"github.com/t0ul/emaildrop/pkg/durable"
)

// Deterministic, no model. A naive retry double-charges; the durable ledger fires
// the refund exactly once on success, replays a retry of a completed action, and
// still lets a genuinely failed attempt be retried to completion.

func TestRetryDoublesUndefended(t *testing.T) {
	if !retryDoublesUndefended() {
		t.Fatal("a naive retry must double the side effect; otherwise there is nothing to make exactly-once")
	}
}

func TestDurableExactlyOnceAndRetryable(t *testing.T) {
	if !exactlyOnceDefended() {
		t.Fatal("the durable ledger must fire once on a retried success and complete a retried failure")
	}
}

func TestFailureIsNotRecorded(t *testing.T) {
	reset()
	l, _ := durable.Open("")
	calls := 0
	fn := func() (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("transient")
		}
		return issueRefund("o1")
	}
	// A failed attempt must NOT be cached (so it is retryable).
	if _, _, err := l.Do(session, "tool", "k", fn); err == nil {
		t.Fatal("a failing fn must return its error, not be recorded")
	}
	if _, replayed, _ := l.Do(session, "tool", "k", fn); replayed {
		t.Fatal("a previously-failed key must re-run fn, not replay")
	}
	if len(paid) != 1 {
		t.Fatalf("the retry must complete exactly once, paid=%d", len(paid))
	}
}
