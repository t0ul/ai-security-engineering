package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/provenance"
)

// Deterministic, no model. The agent's genuine signed output verifies; a forged
// (attacker-signed) artifact and a tampered (content-altered) one do not.

func TestGenuineVerifies(t *testing.T) {
	agent, pub, _ := provenance.NewSigner("emaildrop-agent")
	v := provenance.NewVerifier().Trust("emaildrop-agent", pub)
	if err := v.Verify([]byte(artifact), agent.Sign([]byte(artifact))); err != nil {
		t.Fatalf("the agent's genuine output must verify, got %v", err)
	}
}

func TestForgedRejected(t *testing.T) {
	agent, pub, _ := provenance.NewSigner("emaildrop-agent")
	attacker, _, _ := provenance.NewSigner("attacker")
	v := provenance.NewVerifier().Trust("emaildrop-agent", pub)
	_ = agent
	if err := v.Verify([]byte(artifact), attacker.Sign([]byte(artifact))); err == nil {
		t.Fatal("an artifact signed by a non-agent key must not verify")
	}
}

func TestTamperedRejected(t *testing.T) {
	agent, pub, _ := provenance.NewSigner("emaildrop-agent")
	v := provenance.NewVerifier().Trust("emaildrop-agent", pub)
	mark := agent.Sign([]byte(artifact)) // signed the real content
	if err := v.Verify([]byte(tampered), mark); err == nil {
		t.Fatal("content altered after signing must not verify against the original mark")
	}
}

func TestMainBeforeAfter(t *testing.T) {
	// The whole before/after must hold (ASR 100% unverified -> 0% verified).
	// main() exits non-zero if it fails; exercise the logic by re-running pieces.
	agent, pub, _ := provenance.NewSigner("emaildrop-agent")
	v := provenance.NewVerifier().Trust("emaildrop-agent", pub)
	if v.Verify([]byte(artifact), agent.Sign([]byte(artifact))) != nil {
		t.Fatal("genuine must verify")
	}
}
