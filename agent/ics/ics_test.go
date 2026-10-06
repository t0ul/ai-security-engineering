package ics_test

import (
	"strings"
	"testing"

	"github.com/t0ul/ai-security-engineering/agent/ics"
	"github.com/t0ul/ai-security-engineering/agent/schema"
)

func TestSanitizeFieldStripsExfil(t *testing.T) {
	clean, n := ics.SanitizeField("Join https://evil.example/beacon.gif or ftp://x.y/z now")
	if n != 2 {
		t.Fatalf("removed count = %d, want 2", n)
	}
	if strings.Contains(clean, "evil.example") || strings.Contains(clean, "ftp://") {
		t.Errorf("links not removed: %q", clean)
	}
	clean, n = ics.SanitizeField("click javascript:alert(1)")
	if n != 1 || strings.Contains(clean, "javascript:") {
		t.Errorf("scheme not blocked: %q (n=%d)", clean, n)
	}
	if _, n := ics.SanitizeField("Totally normal location, Room 205"); n != 0 {
		t.Errorf("clean field flagged, n=%d", n)
	}
}

func TestWriteAllDayAndTimed(t *testing.T) {
	events := []schema.Event{
		{Title: "Picture Day", Start: "2026-10-10", AllDay: true, Confidence: 1},
		{Title: "Back to School Night", Start: "2026-09-29T17:30:00", End: "2026-09-29T20:00:00",
			Location: "Gym", Confidence: 1, Warnings: []string{"weekday_mismatch: stated Thursday, but 2026-09-29 is Tuesday"}},
	}
	out, removed, err := ics.Write(events, "Email-to-Calendar")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 0 {
		t.Errorf("unexpected removals: %d", removed)
	}
	for _, want := range []string{
		"BEGIN:VCALENDAR", "VERSION:2.0",
		"DTSTART;VALUE=DATE:20261010",
		"DTSTART:20260929T173000", "DTEND:20260929T200000",
		"SUMMARY:Back to School Night", "LOCATION:Gym",
		"END:VCALENDAR",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ics missing %q", want)
		}
	}
	// warnings land in DESCRIPTION, with the comma escaped per RFC 5545
	if !strings.Contains(out, `DESCRIPTION:⚠ weekday_mismatch: stated Thursday\, but`) {
		t.Errorf("description not escaped as expected:\n%s", out)
	}
	if !strings.HasSuffix(out, "\r\n") || !strings.Contains(out, "\r\n") {
		t.Error("expected CRLF line endings")
	}
	if strings.Count(out, "\r\nBEGIN:VEVENT\r\n") != 2 {
		t.Errorf("expected exactly 2 VEVENTs, got %d", strings.Count(out, "\r\nBEGIN:VEVENT\r\n"))
	}
}

func TestWriteStripsURLInTitleAndCountsIt(t *testing.T) {
	events := []schema.Event{{Title: "RSVP at https://track.evil/pixel", Start: "2026-10-10", AllDay: true, Confidence: 1}}
	out, removed, err := ics.Write(events, "cal")
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if strings.Contains(out, "track.evil") {
		t.Errorf("url survived into SUMMARY:\n%s", out)
	}
	if !strings.Contains(out, "[link removed]") {
		t.Error("expected [link removed] placeholder")
	}
}

func TestWriteDefeatsLineInjection(t *testing.T) {
	// A hostile title that tries to inject its own calendar lines must be folded
	// to a single escaped line, not break out into new iCalendar content.
	events := []schema.Event{{
		Title: "Meeting\r\nBEGIN:VEVENT\r\nSUMMARY:Injected",
		Start: "2026-10-10", AllDay: true, Confidence: 1,
	}}
	out, _, err := ics.Write(events, "cal")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "\r\nBEGIN:VEVENT\r\n") != 1 {
		t.Fatalf("line injection created extra VEVENTs:\n%s", out)
	}
	if !strings.Contains(out, `SUMMARY:Meeting\nBEGIN:VEVENT\nSUMMARY:Injected`) {
		t.Errorf("newlines not escaped to literal \\n:\n%s", out)
	}
}

func TestWriteRejectsBadDate(t *testing.T) {
	events := []schema.Event{{Title: "x", Start: "not-a-date", AllDay: false, Confidence: 1}}
	if _, _, err := ics.Write(events, "cal"); err == nil {
		t.Error("expected an error for an unparseable start")
	}
}
