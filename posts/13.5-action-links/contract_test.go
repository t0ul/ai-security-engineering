package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/hitl"
	"github.com/t0ul/emaildrop/pkg/netpolicy"
)

// Deterministic, no model. A naive agent follows an exfil or metadata action link;
// the stacked human gate, egress policy, and sandbox fetch refuse every variant
// while a genuine approved, allowlisted RSVP still goes through. The exfil and
// metadata URLs are benign markers; nothing is actually fetched.

func TestActionLinkExfilLandsUndefended(t *testing.T) {
	if !attackLandsUndefended() {
		t.Fatal("a naive agent must follow the exfil and metadata links; otherwise there is nothing to defend")
	}
}

func TestStackRefusesEveryVariantAllowsGenuine(t *testing.T) {
	if !stackRefuses() {
		t.Fatal("the stack must refuse auto-click, exfil, and metadata links, and still allow a genuine approved RSVP")
	}
}

func TestEachLegIndependently(t *testing.T) {
	p := pol()
	// HITL: no echoed nonce is refused.
	req := hitl.NewRequest("x", "evidence")
	if ok, _ := actGuarded(req, "", p, legitURL); ok {
		t.Fatal("hitl must refuse an action with no approved nonce")
	}
	// netpolicy: an allowlisted host passes, a non-allowlisted one does not.
	if err := p.Check(legitURL); err != nil {
		t.Fatalf("the allowlisted host should pass netpolicy, got %v", err)
	}
	if err := p.Check(exfilURL); err == nil {
		t.Fatal("netpolicy must refuse a non-allowlisted exfil host")
	}
	_ = netpolicy.Policy{}
}
