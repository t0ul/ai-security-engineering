package main

import (
	"testing"
	"time"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/emaildrop/pkg/provenance"
)

// Deterministic, no model. An ungoverned prompt plane lets anyone swap the agent's
// instructions; the governed plane applies a change only for a signed capability
// that covers write on the prompts resource, and refuses a narrower role.

func TestUngovernedSwapLands(t *testing.T) {
	if !swapLandsUndefended() {
		t.Fatal("an ungoverned plane must let a swap strip the safety rule; otherwise there is nothing to govern")
	}
}

func TestGovernedPlaneRefusesAndRecords(t *testing.T) {
	if !governedDefended() {
		t.Fatal("the governed plane must refuse the app role, allow the operator, keep the rule, and record who did what over a verified chain")
	}
}

func TestOnlyWritePromptsScopeCovers(t *testing.T) {
	signer, pub, _ := provenance.NewSigner("cp")
	authz := controlplane.NewAuthority(signer, provenance.NewVerifier().Trust("cp", pub))
	want := controlplane.Capability{Action: controlplane.ActionWrite, Resource: "prompts", Tenant: "public"}

	writePrompts, _ := authz.IssueScoped(controlplane.Capability{Subject: "op", Action: controlplane.ActionWrite, Resource: "prompts", Tenant: "public"}, nil, time.Hour)
	if _, err := authz.Verify(writePrompts, want); err != nil {
		t.Fatalf("a write/prompts grant must cover a prompt change, got %v", err)
	}
	// A write grant on a DIFFERENT resource must not cover prompts.
	writeOther, _ := authz.IssueScoped(controlplane.Capability{Subject: "op", Action: controlplane.ActionWrite, Resource: "corpus", Tenant: "public"}, nil, time.Hour)
	if _, err := authz.Verify(writeOther, want); err == nil {
		t.Fatal("a write grant on another resource must not cover a prompt change")
	}
}
