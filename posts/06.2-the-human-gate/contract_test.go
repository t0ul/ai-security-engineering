package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/hitl"
)

// Deterministic, no model. A bare approval acts on the naive path; the gate refuses
// an approval that cannot echo the nonce, and only a genuine echo acts.

func TestBareApprovalActsUndefended(t *testing.T) {
	calendar = nil
	if !naiveAcceptsAnything() {
		t.Fatal("a one-click accept should act on a bare approval; otherwise there is nothing to fix")
	}
}

func TestGateRefusesForgedAllowsReal(t *testing.T) {
	calendar = nil
	if !gateHoldsButLetsRealThrough() {
		t.Fatal("the gate must refuse a nonce-less/forged approval and act only on a genuine echo")
	}
}

func TestEvidenceIsRequired(t *testing.T) {
	// An approval with no evidence to review cannot be confirmed at all.
	req := hitl.NewRequest("do a thing") // no evidence
	if ok, _ := req.Confirm(req.Nonce(), true); ok {
		t.Fatal("an evidence-free approval must not be confirmable")
	}
}
