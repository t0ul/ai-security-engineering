package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/agent/items"
	"github.com/t0ul/emaildrop/pkg/agent/schema"
)

// Deterministic, no model. A phishing link wrapped in to-do language is filed as a
// silent task by a sounds-like classifier, and surfaced as an action with its URL
// by the agent's real, URL-first classifier. The link is a benign marker.

func TestSmuggledLinkHiddenUndefended(t *testing.T) {
	if !hiddenUndefended() {
		t.Fatal("the naive classifier must file the URL-bearing item as a task with no surfaced URL; otherwise there is nothing to fix")
	}
}

func TestRealClassifierSurfacesAction(t *testing.T) {
	if !surfacedDefended() {
		t.Fatal("the real classifier must tag the link as an action with its URL and keep the genuine reminder a task")
	}
}

func TestURLAlwaysWinsOverTaskWords(t *testing.T) {
	// A single sentence that is BOTH a chore and a link is an action, never a task.
	out := items.Classify("Reminder: submit the form at "+phishURL+" by Friday.", 2026)
	it, ok := linkItem(out)
	if !ok || it.ResolvedKind() != schema.KindAction || it.URL != phishURL {
		t.Fatalf("a URL-bearing chore must classify as an action with its URL, got %+v", out)
	}
}
