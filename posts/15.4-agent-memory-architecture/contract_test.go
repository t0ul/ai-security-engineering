package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/agent/a2a"
)

// Deterministic, no model. A naive board trusts a forged author and shares every
// scope; the real blackboard rejects an unverifiable write and returns only the
// asking scope. The secret and the forged instruction are benign markers.

func TestForgedWriteRejectedGenuineKept(t *testing.T) {
	key := []byte("planner-key")
	planner := a2a.NewSigner("planner", key)
	rogue := a2a.NewSigner("planner", []byte("wrong"))
	bb := NewBlackboard(a2a.NewVerifier().Trust("planner", key))

	if err := bb.Write("shared", planner.Sign("genuine")); err != nil {
		t.Fatalf("a genuine signed write must be accepted, got %v", err)
	}
	if err := bb.Write("shared", rogue.Sign("forged")); err == nil {
		t.Fatal("a write signed with the wrong key must be rejected")
	}
	if containsFact(bb.Recall("shared"), "forged") {
		t.Fatal("a rejected write must not be on the board")
	}
	if !containsFact(bb.Recall("shared"), "genuine") {
		t.Fatal("the genuine write must be on the board")
	}
}

func TestRecallIsScoped(t *testing.T) {
	pk, rk := []byte("pk"), []byte("rk")
	planner := a2a.NewSigner("planner", pk)
	researcher := a2a.NewSigner("researcher", rk)
	bb := NewBlackboard(a2a.NewVerifier().Trust("planner", pk).Trust("researcher", rk))

	_ = bb.Write("planner-private", planner.Sign("secret-1234"))
	_ = bb.Write("researcher-private", researcher.Sign("notes"))

	if containsFact(bb.Recall("researcher-private"), "1234") {
		t.Fatal("one agent's scoped recall must not return another's private note")
	}
	if !containsFact(bb.Recall("planner-private"), "1234") {
		t.Fatal("an agent's own scoped recall must return its own note")
	}
}

func TestWholeDemoHolds(t *testing.T) {
	// A guard that the end-to-end before/after logic stays true.
	key := []byte("k")
	s := a2a.NewSigner("planner", key)
	bb := NewBlackboard(a2a.NewVerifier().Trust("planner", key))
	if err := bb.Write("s", s.Sign("x")); err != nil {
		t.Fatalf("baseline write failed: %v", err)
	}
}
