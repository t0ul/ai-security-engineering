package extractor

import (
	"strings"
	"testing"
)

// TestTimeInText covers the clock-plausibility matcher behind the fabricated-time fix.
func TestTimeInText(t *testing.T) {
	cases := []struct {
		iso, raw string
		want     bool
	}{
		{"2026-09-29T18:00:00", "Back to School Night at 6:00 PM", true}, // 12h form
		{"2026-09-29T18:00:00", "doors open 18:00", true},               // 24h form
		{"2026-09-29T18:00:00", "join us at 6 pm", true},                // bare hour
		{"2026-09-29T22:27:00", "fire drills this week", false},         // fabricated, absent
		{"2026-09-29T09:30:00", "meeting at 9:30 am", true},
		{"2026-09-29T09:30:00", "no time stated here", false},
	}
	for _, c := range cases {
		if got := timeInText(c.iso, c.raw); got != c.want {
			t.Errorf("timeInText(%q,%q)=%v want %v", c.iso, c.raw, got, c.want)
		}
	}
}

// TestCandidatesDropFabricatedTime locks the A1 time fix: an LLM candidate whose parsed
// time is NOT written in the email is coerced to all-day (no bogus precise time), while a
// real stated time is kept. Drives candidatesToEvents directly — no live model.
func TestCandidatesDropFabricatedTime(t *testing.T) {
	// Fabricated: the phrase carries a stray 22:27 the email body never states.
	raw := "PS 123\nSafety drills are scheduled for Monday, October 12th.\n"
	ev := candidatesToEvents([]candidate{{Title: "drills", Phrase: "Monday October 12th at 22:27"}}, "x.txt", 2026, raw)
	if len(ev) != 1 {
		t.Fatalf("want 1 event, got %d", len(ev))
	}
	if !ev[0].AllDay {
		t.Errorf("fabricated time must coerce to all-day, got Start=%q AllDay=%v", ev[0].Start, ev[0].AllDay)
	}
	if !hasWarn(ev[0].Warnings, "implausible time") {
		t.Errorf("want an implausible-time warning, got %v", ev[0].Warnings)
	}

	// Legit: the time is actually written, so it survives as a timed event.
	raw2 := "Back to School Night: Thursday, September 29th at 6:00 PM in the auditorium\n"
	ev2 := candidatesToEvents([]candidate{{Title: "Back to School Night", Phrase: "Thursday September 29th at 6:00 PM"}}, "x.txt", 2026, raw2)
	if len(ev2) != 1 {
		t.Fatalf("want 1 event, got %d", len(ev2))
	}
	if ev2[0].AllDay {
		t.Errorf("a stated time must be kept, got all-day (Start=%q)", ev2[0].Start)
	}
	if !strings.Contains(ev2[0].Start, "T18:00") {
		t.Errorf("want 18:00 start, got %q", ev2[0].Start)
	}
}

func hasWarn(ws []string, sub string) bool {
	for _, w := range ws {
		if strings.Contains(w, sub) {
			return true
		}
	}
	return false
}
