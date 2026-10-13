package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/t0ul/emaildrop/pkg/durable"
)

// Deterministic, no model. A naive resume trusts a forged checkpoint and reverses a
// decision; reopening the hash-chained log fails closed on a tampered checkpoint,
// while a clean log resumes with the real decision intact.

func TestNaiveResumeAdoptsForgedState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "steps.log")
	runWorkflow(path)
	tamper(path)
	if naiveResume(path) != "approved" {
		t.Fatal("a naive resume must adopt the tampered decision; otherwise there is nothing to defend")
	}
}

func TestOpenFailsClosedOnTamper(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "steps.log")
	runWorkflow(path)
	tamper(path)
	if _, err := durable.Open(path); !errors.Is(err, durable.ErrTampered) {
		t.Fatalf("reopening a tampered log must fail with ErrTampered, got %v", err)
	}
}

func TestCleanLogResumesWithRealDecision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "steps.log")
	runWorkflow(path)
	l, err := durable.Open(path)
	if err != nil {
		t.Fatalf("a clean log must resume, got %v", err)
	}
	if s, ok := l.Lookup(session, "approve"); !ok || s.Output != "denied" {
		t.Fatalf("the real approval decision must survive resume, got %q (ok=%v)", s.Output, ok)
	}
	_ = os.Remove(path)
}
