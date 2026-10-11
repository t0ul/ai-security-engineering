// Command main is the runnable demo for lesson 01.2, dates the model never decides.
//
// Run it with:
//
//	go run .
//
// Before: a naive parser takes the month and the day from the email, never looks
// at the stated weekday, and lets time.Date normalize an impossible date, so both
// inputs land on the calendar as wrong dates with no warning attached.
//
// After: the agent's real date parser (github.com/t0ul/emaildrop/pkg/agent/dateparse)
// flags the weekday mismatch so the event is held for a human, and rejects the
// impossible date outright. A clean date still gets through.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/t0ul/emaildrop/pkg/agent/dateparse"
	"github.com/t0ul/emaildrop/pkg/agent/ics"
	"github.com/t0ul/emaildrop/pkg/agent/schema"
)

// year is the school year the emails belong to; they almost never say it.
const year = 2026

// attacks are the two inputs this lesson runs (see attack/README.md).
var attacks = []string{"attack/back-to-school-night.txt", "attack/february-30.txt"}

// clean is a correct line from the same public sample, to show the control
// still calendars a good date. September 29, 2026 really is a Tuesday.
var clean = snippet{Title: "Back-to-School Night", When: "Tuesday, September 29th 5:30PM - 8:00PM", Where: "School Building"}

// snippet is one event as it appears in an email: a title, a date line, a place.
type snippet struct{ Title, When, Where string }

func load(path string) snippet {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	s := snippet{Title: strings.TrimSpace(lines[0])}
	if len(lines) > 1 {
		s.When = strings.TrimSpace(lines[1])
	}
	if len(lines) > 2 {
		s.Where = strings.TrimSpace(lines[2])
	}
	return s
}

func main() {
	fmt.Println("== before: no defense ==")
	if attackUndefended() {
		fmt.Println("attack landed (this is the problem the post fixes)")
	} else {
		fmt.Println("the undefended path should have calendared both wrong dates")
		os.Exit(1)
	}

	fmt.Println("\n== after: with the control ==")
	if attackDefended() {
		fmt.Println("attack blocked (the receipt)")
	} else {
		fmt.Println("the defended path should have kept both wrong dates off the calendar")
		os.Exit(1)
	}

	fmt.Println("\n== still works: a clean date ==")
	if !cleanStillCalendared() {
		fmt.Println("the control should still calendar a correct date")
		os.Exit(1)
	}
}

// ---- before: the parser I would have written first ----

var months = map[string]int{
	"january": 1, "february": 2, "march": 3, "april": 4, "may": 5, "june": 6,
	"july": 7, "august": 8, "september": 9, "october": 10, "november": 11, "december": 12,
}

var (
	reNaiveDate = regexp.MustCompile(`(?i)\b(January|February|March|April|May|June|July|August|September|October|November|December)\s+(\d{1,2})`)
	reNaiveTime = regexp.MustCompile(`(?i)\b(\d{1,2})(?::(\d{2}))?\s*(AM|PM)\b`)
)

// naiveStart finds the month, the day, and the first clock time, and hands them
// to time.Date. It never looks at the weekday the email states, and time.Date
// quietly normalizes February 30 into March 2. This is the undefended path.
func naiveStart(when string) (string, bool) {
	m := reNaiveDate.FindStringSubmatch(when)
	if m == nil {
		return "", false
	}
	month := months[strings.ToLower(m[1])]
	day, _ := strconv.Atoi(m[2])
	hour, minute := 0, 0
	if t := reNaiveTime.FindStringSubmatch(when); t != nil {
		hour, _ = strconv.Atoi(t[1])
		if t[2] != "" {
			minute, _ = strconv.Atoi(t[2])
		}
		if strings.EqualFold(t[3], "PM") && hour < 12 {
			hour += 12
		}
		if strings.EqualFold(t[3], "AM") && hour == 12 {
			hour = 0
		}
	}
	d := time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC)
	return d.Format("2006-01-02T15:04:05"), true
}

// attackUndefended runs both inputs through the naive parser and writes what it
// produced with the agent's real calendar writer. It returns true when both wrong
// dates were calendared with no warning attached, which is the attack landing.
func attackUndefended() bool {
	landed := 0
	for _, path := range attacks {
		s := load(path)
		start, ok := naiveStart(s.When)
		if !ok {
			fmt.Printf("%s\n  could not parse %q\n", path, s.When)
			continue
		}
		ev := schema.New(s.Title, start)
		ev.Location = s.Where
		text, _, err := ics.Write([]schema.Event{ev}, "lesson-01.2")
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s\n  email says:  %s\n  calendared:  %s  (%s; warnings: %d)\n",
			path, s.When, dtstart(text), weekdayOf(start), len(ev.Warnings))
		if len(ev.Warnings) == 0 {
			landed++
		}
	}
	return landed == len(attacks)
}

// ---- after: the agent's real date parser ----

// defend runs one snippet through dateparse and decides what happens to it.
// It returns the .ics text when the event was calendared, or "" when it was
// rejected or held for review.
func defend(s snippet) (outcome string, calendared string) {
	r := dateparse.ExtractDatetime(s.When, year)
	if r == nil {
		return "rejected: no such date, nothing to calendar", ""
	}
	ev := schema.New(s.Title, r.Start)
	ev.End, ev.AllDay, ev.Location = r.End, r.AllDay, s.Where
	ev.Warnings = integrity(r.Warnings)
	if ev.NeedsReview() {
		return "held for review, not calendared", ""
	}
	text, _, err := ics.Write([]schema.Event{ev}, "lesson-01.2")
	if err != nil {
		panic(err)
	}
	return "calendared", text
}

// integrity keeps the warnings that mean the data is suspect and drops the
// informational assumed_year note, exactly as the product's extractor does:
// the emails never state the year, so assuming it is not a data problem.
func integrity(warnings []string) []string {
	out := []string{}
	for _, w := range warnings {
		if !strings.HasPrefix(w, "assumed_year") {
			out = append(out, w)
		}
	}
	return out
}

// attackDefended runs the same two inputs through the agent's real parser. It
// returns true when both were kept off the calendar: the weekday mismatch held
// for a human, the impossible date rejected.
func attackDefended() bool {
	blocked := 0
	for _, path := range attacks {
		s := load(path)
		r := dateparse.ExtractDatetime(s.When, year)
		outcome, text := defend(s)
		fmt.Printf("%s\n  email says:  %s\n", path, s.When)
		if r != nil {
			fmt.Printf("  parsed:      %s\n  warnings:    %s\n", r.Start, strings.Join(integrity(r.Warnings), "; "))
		}
		fmt.Printf("  %s\n", outcome)
		if text == "" {
			blocked++
		}
	}
	return blocked == len(attacks)
}

// cleanStillCalendared shows the control is not just refusing everything: a
// correct line from the same newsletter goes straight through to the calendar.
func cleanStillCalendared() bool {
	outcome, text := defend(clean)
	fmt.Printf("  email says:  %s\n  %s", clean.When, outcome)
	if text != "" {
		fmt.Printf(":  %s  (%s)", dtstart(text), weekdayOf(dtstartISO(text)))
	}
	fmt.Println()
	return text != ""
}

// ---- small helpers for printing ----

// dtstart returns the DTSTART line from .ics text, the field a calendar app reads.
func dtstart(icsText string) string {
	for _, line := range strings.Split(icsText, "\r\n") {
		if strings.HasPrefix(line, "DTSTART") {
			return line
		}
	}
	return "(no DTSTART)"
}

// dtstartISO converts the DTSTART value back to the ISO form dateparse uses.
func dtstartISO(icsText string) string {
	v := strings.TrimPrefix(dtstart(icsText), "DTSTART:")
	if t, err := time.Parse("20060102T150405", v); err == nil {
		return t.Format("2006-01-02T15:04:05")
	}
	return v
}

// weekdayOf says what day of the week an ISO start really falls on.
func weekdayOf(isoStart string) string {
	t, err := time.Parse("2006-01-02T15:04:05", isoStart)
	if err != nil {
		return isoStart
	}
	return t.Format("2006-01-02 is a Monday")
}
