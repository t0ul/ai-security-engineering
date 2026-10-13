package main

import (
	"errors"
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
)

// Deterministic, no model. An expiry-only check honors a revoked token; the real
// authority refuses it the instant it is revoked, still honors a fresh grant, and
// the kill switch revokes every grant at once.

func TestRevokedTokenStillWorksUndefended(t *testing.T) {
	if !revokedStillWorksUndefended() {
		t.Fatal("an expiry-only check must still honor a revoked token; otherwise there is nothing to fix")
	}
}

func TestRevocationAndKillSwitchHold(t *testing.T) {
	if !revocationHoldsDefended() {
		t.Fatal("the authority must refuse a revoked grant, honor a fresh one, and refuse everything under the kill switch")
	}
}

func TestReinstateRestores(t *testing.T) {
	authz, issue := newAuthority()
	g := issue("agent")
	authz.Revoke("agent")
	if _, err := authz.Verify(g, want); !errors.Is(err, controlplane.ErrGrantRevoked) {
		t.Fatalf("a revoked grant must be refused, got %v", err)
	}
	authz.Reinstate("agent")
	if _, err := authz.Verify(g, want); err != nil {
		t.Fatalf("a reinstated subject's grant must verify again, got %v", err)
	}
}
