package server

import "testing"

func TestDedupCollapsesFragmentTitle(t *testing.T) {
	in := []Event{
		{Title: "The Science Fair is", Start: "2026-10-15 18:00", Kind: "event"},
		{Title: "Science Fair", Start: "2026-10-15 18:00", Kind: "event"},
	}
	got := dedupEvents(in)
	if len(got) != 1 {
		t.Fatalf("near-dup same-slot events should collapse to 1, got %d: %+v", len(got), got)
	}
	if got[0].Title != "Science Fair" {
		t.Fatalf("the cleaner title should win, got %q", got[0].Title)
	}
}

func TestDedupKeepsDistinctSameDayEvents(t *testing.T) {
	in := []Event{
		{Title: "Science Fair", Start: "2026-10-15 18:00", Kind: "event"},
		{Title: "Band Concert", Start: "2026-10-15 18:00", Kind: "event"}, // same slot, unrelated title
		{Title: "Science Fair", Start: "2026-10-16 18:00", Kind: "event"}, // same title, different day
	}
	if got := dedupEvents(in); len(got) != 3 {
		t.Fatalf("distinct events must not be merged, got %d: %+v", len(got), got)
	}
}

func TestDedupSeparatesEventFromTask(t *testing.T) {
	in := []Event{
		{Title: "Permission slip", Start: "2026-10-15", Kind: "event"},
		{Title: "Permission slip", Due: "2026-10-15", Kind: "task"},
	}
	if got := dedupEvents(in); len(got) != 2 {
		t.Fatalf("an event and a task must stay distinct, got %d", len(got))
	}
}
