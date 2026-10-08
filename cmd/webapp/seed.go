package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/t0ul/ai-security-engineering/pkg/datastore"
)

// seedEmails ingests the example emails into the inbox on first run, so a fresh drop
// folder isn't empty and the operator doesn't have to re-drag emails every boot. The
// running watcher then processes them into persisted .ics in the outbox, which is the
// source of truth read on every request — so this happens exactly once per drop.
//
// Idempotent two ways: a DB marker ("emails_seeded") set after a successful seed, and
// a non-empty outbox (a drop already populated by prior manual drops is left alone).
func seedEmails(inv *datastore.Store, inboxDir, outboxDir, seedDir string) {
	if inv == nil || seedDir == "" {
		return
	}
	if _, ok, _ := inv.GetConfig("emails_seeded"); ok {
		return // already seeded this drop
	}
	if hasICS(outboxDir) {
		_ = inv.SetConfig("emails_seeded", "preexisting") // already populated by prior drops
		return
	}
	files, _ := filepath.Glob(filepath.Join(seedDir, "*.txt"))
	if len(files) == 0 {
		return // no seed source; don't mark, so pointing -seed at a real dir later still works
	}
	if err := os.MkdirAll(inboxDir, 0o755); err != nil {
		log.Printf("webapp: seed inbox: %v", err)
		return
	}
	n := 0
	for _, f := range files {
		base := filepath.Base(f)
		if strings.Contains(strings.ToLower(base), "handbook") {
			continue // a knowledge-base doc, not a calendar email
		}
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if os.WriteFile(filepath.Join(inboxDir, base), data, 0o644) == nil {
			n++
		}
	}
	_ = inv.SetConfig("emails_seeded", strconv.Itoa(n))
	log.Printf("webapp: seeded %d example emails into inbox (first run); the watcher will process them into %s", n, outboxDir)
}

// hasICS reports whether a directory already holds processed .ics output.
func hasICS(dir string) bool {
	m, _ := filepath.Glob(filepath.Join(dir, "*.ics"))
	return len(m) > 0
}
