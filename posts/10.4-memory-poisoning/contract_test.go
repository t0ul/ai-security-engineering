package main

import "testing"

// Deterministic, no model. A poisoned note survives in a flat memory and reaches
// the recall prompt; the agent's real memory severs it three ways: scope (another
// subject recalls nothing), TTL (it expires and is never recalled again), and
// sanitized recall (the untrusted note is neutralized while the trusted fact
// survives). The poison is a benign marker instruction.

func TestPoisoningLandsUndefended(t *testing.T) {
	if !poisoningLandsUndefended() {
		t.Fatal("a flat forever-memory must carry the poisoned note into recall; otherwise there is nothing to defend")
	}
}

func TestLifecycleSeversIt(t *testing.T) {
	if !lifecycleSeversIt() {
		t.Fatal("scoped, expiring, sanitized memory must keep the poison out of recall while preserving the trusted fact and the asker's own scope")
	}
}
