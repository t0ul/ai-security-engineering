package main

import (
	"strings"
	"testing"

	"github.com/t0ul/emaildrop/pkg/agent/tool"
	"github.com/t0ul/emaildrop/pkg/agent/tools"
)

// Deterministic, no model. A naive directory that surfaces every recognized entity
// leaks the SSN and the account key; the agent's PII-scoped contacts tool surfaces
// only email addresses, is read-only, and flags what it surfaced. The SSN and key
// are benign markers.

func TestNaiveDirectoryLeaksPII(t *testing.T) {
	if !overSurfacedUndefended() {
		t.Fatal("a dump-everything directory must surface the SSN or key; otherwise there is nothing to scope")
	}
}

func TestContactsToolIsScoped(t *testing.T) {
	if !scopedDefended() {
		t.Fatal("the contacts tool must surface only addresses, read-only, and audit the count")
	}
}

func TestContactsNeverSurfacesSecrets(t *testing.T) {
	res := tools.Contacts{}.Run("key AKIAIOSFODNN7EXAMPLE and ssn 123-45-6789, write to a@b.com", tool.Ctx{})
	if strings.Contains(res.Text, "AKIA") || strings.Contains(res.Text, "123-45-6789") {
		t.Fatalf("the contacts tool must never surface a secret or SSN, got %q", res.Text)
	}
	if res.Capability != tool.ReadOnly {
		t.Fatal("the contacts tool must be read-only (it cannot write or exfiltrate)")
	}
}
