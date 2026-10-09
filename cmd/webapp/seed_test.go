package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSeedEmailsIncremental locks the fix: seeding is incremental + idempotent — it drops
// only example emails not already processed (no .ics in the outbox) and not already queued
// in the inbox, so newly-added example files get picked up while done ones are left alone.
func TestSeedEmailsIncremental(t *testing.T) {
	base := t.TempDir()
	seed := filepath.Join(base, "examples")
	inbox := filepath.Join(base, "inbox")
	outbox := filepath.Join(base, "outbox")
	for _, d := range []string{seed, inbox, outbox} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(dir, name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 1 already processed (has .ics), 2 queued in inbox, 3 brand new, plus a handbook.
	write(seed, "1.txt", "old")
	write(seed, "2.txt", "queued")
	write(seed, "3.txt", "new email")
	write(seed, "family-handbook.txt", "reference doc")
	write(outbox, "1.events.ics", "BEGIN:VCALENDAR")
	write(inbox, "2.txt", "already queued")

	seedEmails(nil, inbox, outbox, seed)

	inboxHas := func(name string) bool { _, err := os.Stat(filepath.Join(inbox, name)); return err == nil }
	if inboxHas("1.txt") {
		t.Error("an already-processed email must NOT be re-seeded")
	}
	if !inboxHas("3.txt") {
		t.Error("a new example email must be seeded into the inbox")
	}
	if inboxHas("family-handbook.txt") {
		t.Error("a handbook must not be seeded as an email")
	}
	// 2.txt was already in the inbox — still there, not duplicated/overwritten oddly.
	if b, _ := os.ReadFile(filepath.Join(inbox, "2.txt")); string(b) != "already queued" {
		t.Error("a queued email must be left untouched")
	}

	// Idempotent: a second run seeds nothing new.
	write(outbox, "3.events.ics", "x") // pretend 3 got processed
	seedEmails(nil, inbox, outbox, seed)
	// 3.txt may still be in inbox from the first run; the point is no error + no re-copy of 1.
	if inboxHas("1.txt") {
		t.Error("second run must still skip the processed email")
	}
}
