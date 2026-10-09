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

// TestDedupCollapsesNearDuplicateWording: the same holiday phrased differently across
// emails collapses, but distinct same-day events with long shared prefixes do not.
func TestDedupCollapsesNearDuplicateWording(t *testing.T) {
	in := []Event{
		{Title: "Italian Heritage/Indigenous Peoples' Day", Start: "2026-10-12", AllDay: true, Kind: "event"},
		{Title: "Italian Heritage and Indigenous Peoples' Day, schools closed", Start: "2026-10-12", AllDay: true, Kind: "event"},
	}
	if got := dedupEvents(in); len(got) != 1 {
		t.Fatalf("near-duplicate wording of the same event must collapse, got %d: %+v", len(got), got)
	}
	// Distinct conferences on the SAME day (shared prefix) must NOT merge.
	in2 := []Event{
		{Title: "Evening Parent-Teacher Conferences for middle schools and D75", Start: "2026-11-05", AllDay: true, Kind: "event"},
		{Title: "Evening Parent-Teacher Conferences for high schools, K-12, and 6-12 schools", Start: "2026-11-05", AllDay: true, Kind: "event"},
	}
	if got := dedupEvents(in2); len(got) != 2 {
		t.Fatalf("distinct same-day conferences must stay separate, got %d: %+v", len(got), got)
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

// TestDedupCollapsesSameDayDifferentTime is the per-day fix: the same event on the
// same day with a different (often fabricated) time collapses to one, and the
// all-day version wins over the mis-timed one.
func TestDedupCollapsesSameDayDifferentTime(t *testing.T) {
	in := []Event{
		{Title: "Evacuation Drill", Start: "2026-10-14 22:27", Kind: "event"},
		{Title: "Evacuation Drill", Start: "2026-10-14", Kind: "event", AllDay: true},
	}
	got := dedupEvents(in)
	if len(got) != 1 {
		t.Fatalf("same event, same day should collapse to 1, got %d: %+v", len(got), got)
	}
	if !got[0].AllDay {
		t.Fatalf("the all-day entry should win over the mis-timed one, got %+v", got[0])
	}
}

// TestDropFragments removes lowercase-start sentence fragments but keeps real titles.
func TestDropFragments(t *testing.T) {
	in := []Event{
		{Title: "went smoothly. See below for the dates", Start: "2026-09-15", Kind: "event"},
		{Title: "Our first evacuation drill", Start: "2026-09-15", Kind: "event"},
		{Title: "3-407 visits the Columbus Branch", Start: "2026-10-08", Kind: "event"},
	}
	got := dropFragments(in)
	if len(got) != 2 {
		t.Fatalf("one fragment should be dropped, got %d: %+v", len(got), got)
	}
	for _, e := range got {
		if e.Title[0] >= 'a' && e.Title[0] <= 'z' {
			t.Fatalf("a fragment survived: %q", e.Title)
		}
	}
}
