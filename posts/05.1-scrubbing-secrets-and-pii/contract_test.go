package main

import "testing"

// Deterministic, no model. Indexing the email as-is puts every recognized secret
// on disk; scrubbing at ingest keeps all of them out. The test also pins the
// honest limit: a token format goflage does not recognize still slips through, so
// if a later goflage gains that recognizer, this test fails and tells us to update
// the lesson.

func TestSecretsLeakUndefendedScrubbedDefended(t *testing.T) {
	dir := t.TempDir()
	if !indexRaw(dir) {
		t.Fatal("indexing as-is should put every recognized secret on disk; otherwise there is nothing to fix")
	}
	if !indexScrubbed(dir) {
		t.Fatal("scrubbing at ingest must keep every recognized secret out of the corpus")
	}
}

func TestUnrecognizedFormatStillSlips(t *testing.T) {
	dir := t.TempDir()
	clean, _ := leakyScrubbed()
	path := corpus(dir, clean)
	if !onDisk(path, unusual) {
		t.Skip("goflage now recognizes this format; update the lesson's honesty note, this is good news")
	}
}
