package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
)

// Deterministic, no model. A re-pasted prompt cannot be proven to be the known-good
// version (its hash differs); re-activating the pinned version restores the exact
// content, verifies by hash, and is audited over a verified chain.

func TestRepasteIsUnprovable(t *testing.T) {
	if !unprovableRollbackUndefended() {
		t.Fatal("a re-pasted prompt must fail the hash pin; otherwise there is no drift to detect")
	}
}

func TestCleanRollbackRestoresAndAudits(t *testing.T) {
	if !cleanRollbackDefended() {
		t.Fatal("re-activating the pinned version must restore it provably and record the rollback")
	}
}

func TestReactivatingGoodTextMatchesPinnedHash(t *testing.T) {
	p := newPlane()
	good := p.Get(name)
	p.Activate(name, badPrompt)
	if p.Verify(name, good.Hash) {
		t.Fatal("the bad change must not match the known-good hash")
	}
	v := p.Activate(name, good.Text)
	if !p.Verify(name, good.Hash) {
		t.Fatalf("re-activating the exact good text must match its pinned hash; live version %d", v.Version)
	}
	// Rollback is a new version number but the same content hash.
	if v.Version == good.Version {
		t.Fatal("a rollback is a new version, not a mutation of the old one")
	}
	_ = controlplane.HashPrompt
}
