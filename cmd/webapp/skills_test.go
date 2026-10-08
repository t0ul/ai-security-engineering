package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/t0ul/ai-security-engineering/pkg/controlplane"
	"github.com/t0ul/ai-security-engineering/pkg/datastore"
)

// liveSkills wires the real signed catalog to a real governed plane backed by a real
// (throwaway) SQLite file — no mocks, the same objects main.go builds.
func liveSkills(t *testing.T) (*skillLoader, *controlplane.Skills, *datastore.Store) {
	t.Helper()
	inv, err := datastore.Open(filepath.Join(t.TempDir(), "inv.db"))
	if err != nil {
		t.Fatalf("open datastore: %v", err)
	}
	loader, names := newSkillLoader()
	plane := controlplane.GovernedSkills(names, inv, nil)
	loader.attach(plane)
	return loader, plane, inv
}

// TestSkillLoaderFailsClosedThenLoadsOnApprove is the governed consumer end to end:
// an untrusted-signed skill is always refused; a trusted skill is refused until the
// operator approves its exact content, then loads.
func TestSkillLoaderFailsClosedThenLoadsOnApprove(t *testing.T) {
	loader, plane, inv := liveSkills(t)
	defer inv.Close()

	// Attacker-signed skill: refused regardless of approval (untrusted signature).
	if r := loader.Load("free-ical-pro", false); r.Loaded {
		t.Fatalf("attacker-signed skill must never load, got %+v", r)
	} else if !strings.Contains(r.Reason, "signature") {
		t.Fatalf("want signature-refusal reason, got %q", r.Reason)
	}

	// Trusted skill, not yet approved: refused (fail-closed on the pin).
	if r := loader.Load("calendar-helper", false); r.Loaded {
		t.Fatalf("unapproved skill must not load, got %+v", r)
	} else if !strings.Contains(r.Reason, "approved") {
		t.Fatalf("want not-approved reason, got %q", r.Reason)
	}

	// Operator approves the exact catalog content → the pin now matches.
	hash, ok := loader.FullHash("calendar-helper")
	if !ok {
		t.Fatal("catalog missing calendar-helper")
	}
	plane.Approve("calendar-helper", hash)

	if r := loader.Load("calendar-helper", false); !r.Loaded || r.Unsafe || r.Instructions == "" {
		t.Fatalf("approved trusted skill should load through the gate, got %+v", r)
	}
	// Persisted for rehydrate-on-boot.
	if h, ok, _ := inv.LatestSkillPin("calendar-helper"); !ok || h != hash {
		t.Fatalf("approval should persist the pin, ok=%v h=%q", ok, h)
	}
}

// TestSkillLoaderUnsafeLeaksAttack is the controls-off demo invariant: bypassing the
// gate lets a poisoned skill's instructions through verbatim (the attack the control
// exists to stop).
func TestSkillLoaderUnsafeLeaksAttack(t *testing.T) {
	loader, _, inv := liveSkills(t)
	defer inv.Close()

	r := loader.Load("free-ical-pro", true)
	if !r.Loaded || !r.Unsafe {
		t.Fatalf("controls-off load should return unchecked, got %+v", r)
	}
	if !strings.Contains(r.Instructions, "PWNED-SKILL") {
		t.Fatalf("controls-off should leak the poison marker, got %q", r.Instructions)
	}
}

// TestSkillCatalogTrustFlags: the catalog reports the author skill as trusted and the
// attacker skill as untrusted, and reflects approval state.
func TestSkillCatalogTrustFlags(t *testing.T) {
	loader, plane, inv := liveSkills(t)
	defer inv.Close()
	byName := map[string]bool{}
	for _, row := range loader.Catalog() {
		byName[row.Name] = row.SignerTrusted
	}
	if !byName["calendar-helper"] {
		t.Error("author-signed skill should be trusted")
	}
	if byName["free-ical-pro"] {
		t.Error("attacker-signed skill must be untrusted")
	}
	// Approve flips the approved flag in the catalog view.
	h, _ := loader.FullHash("calendar-helper")
	plane.Approve("calendar-helper", h)
	for _, row := range loader.Catalog() {
		if row.Name == "calendar-helper" && !row.Approved {
			t.Error("approved skill should show approved=true")
		}
	}
}
