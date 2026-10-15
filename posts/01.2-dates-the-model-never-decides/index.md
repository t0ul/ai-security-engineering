---
title: "Dates the model never decides"
description: "The first field that bit me was the date. A naive parser happily calendars a Thursday that is really a Tuesday and a February 30 that does not exist, with no warning. This post shows that happening, then adds the agent's deterministic date parser, which flags the first and rejects the second, and proves it with a test you can run."
date: 2025-09-09
draft: false
tags: ["ai-security", "golang", "agents"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The field that bit me first

Of all the fields in [the event struct](/blog/one-shape-for-everything/), the one that caused trouble first was the date. School emails bury dates in prose, almost never say the year, and now and then they get the weekday wrong. That last one sounds harmless until you remember what the agent is for: a wrong date on the family calendar is the whole product failing, and failing quietly, because a calendar entry does not look wrong. It just sits there.

Security usually comes last. I would rather build it in as I go and prove it with a test, and this post is the first time that habit pays for itself, before a model is anywhere near the code. The whole lesson lives in one folder of the book repo, and it runs with nothing else set up:

```bash
cd posts/01.2-dates-the-model-never-decides
go run .
```

The folder pulls the agent's real date parser and calendar writer straight from the product repo at a pinned commit, so you do not need a checkout of emaildrop next to it. The whole program is on this page, in the order it runs, so you can type it in instead. Make an empty folder, run `go mod init dates`, then `go get github.com/t0ul/emaildrop@` followed by the version on the `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/01.2-dates-the-model-never-decides

go 1.27.1

// The agent's real date parser and calendar writer, pulled from the product
// repo at the commit this lesson was written against. No local overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
```
<!-- /listing -->

## The spine of the program

Every lesson in this book is one program with the same shape. The top of the file says what the lesson claims, imports what it needs, and `main` runs the undefended version, the defended version and one more check, printing the receipt only if all of them came out the way the lesson says. This one also declares the year the emails never state, the two input files, a correct line from the same newsletter for the last check, and a `snippet`: one event the way it appears in an email, a title line, a date line, a place line, read from a file by `load`.

<!-- listing: main.go#head -->
```go
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
```
<!-- /listing -->

Three checks, three booleans. If any of them stops coming out the way the lesson claims, the program says so and exits non-zero, which is how the book's board catches a lesson that went red.

## I. A parser that works

Here is the date parser I would have written first, and honestly did, in spirit. Find the month name and the day, find the first clock time, hand them to Go's `time.Date`, format the result. Under it is the undefended run: both inputs through that parser, and whatever it produced handed to the agent's real calendar writer, so what you see is the actual `DTSTART` line a calendar app would read.

<!-- listing: main.go#before: the parser I would have written first -->
```go
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
```
<!-- /listing -->

The two regular expressions just match a month name followed by a number, and a clock time with AM or PM. Nothing clever. Give `naiveStart` a line like `Tuesday, September 29th 5:30PM - 8:00PM` and you get `2026-09-29T17:30:00`, which is right.

Then I fed it two lines. The first is real: it is from one of the public sample newsletters in the product repo, `testdata/emaildrop/samples/6.txt`, exactly as it appears in that sample. The second I made up, because no school has sent me a February 30 yet, but a typo like it is only a matter of time. Both live in the lesson's `attack/` folder; if you are typing along, make that folder and these two files, each three lines:

<!-- listing: attack/back-to-school-night.txt#all -->
```
Back to School Night
Thursday, September 29th from 5:30PM-8PM
School Building
```
<!-- /listing -->

<!-- listing: attack/february-30.txt#all -->
```
Winter Recital
February 30th at 9:00AM
Auditorium
```
<!-- /listing -->

Here is the first half of `go run .`:

```
== before: no defense ==
attack/back-to-school-night.txt
  email says:  Thursday, September 29th from 5:30PM-8PM
  calendared:  DTSTART:20260929T173000  (2026-09-29 is a Tuesday; warnings: 0)
attack/february-30.txt
  email says:  February 30th at 9:00AM
  calendared:  DTSTART:20260302T090000  (2026-03-02 is a Monday; warnings: 0)
attack landed (this is the problem the post fixes)
```

Read those two `calendared` lines slowly. The email said Thursday. The parser took September 29, which in 2026 is a Tuesday, and calendared it without a word, so now there is an entry that disagrees with the email it came from, and nobody knows which one is wrong. Then it took February 30 and calendared March 2. That second one is `time.Date` doing exactly what its documentation says: it normalizes, so the 30th of February becomes the 2nd of March, and the parser passes that along as if the email had said so.

Nobody attacked anything here. A school sent a sloppy line and the code did what it was told. That is the thing I want you to notice: the naive version does not fail, it succeeds at the wrong thing, and the only way you would ever find out is by missing Back to School Night. And once a model is the thing proposing the date text, a few posts from now, you will not be able to tell a typo from a nudge. So the fix has to live in code, below the model, and it has to be the same code every time.

## II. Dates the model never decides

The control is the agent's own date parser, `github.com/t0ul/emaildrop/pkg/agent/dateparse`. It is deterministic parsing plus integrity checking, written against the standard library and nothing else, and its one rule is in its name: the model is never asked to do date arithmetic. A model may later propose the span of text that holds the date, but turning that text into a real date, and checking that the date makes sense, happens here, in code I can read and test, the same way every run.

It does two things the naive parser did not. First, after calling `time.Date` it compares the result's year, month, and day back against what it was given, and if they moved, the date was impossible and the whole thing is rejected rather than normalized. Second, if the email states a weekday, it works out the real weekday of the date and compares the two, and a mismatch becomes a warning in plain words. Here is the lesson's defended path, which is just that parser plus the review rule from the last post, followed by the defended run over the same two files and the check that a correct date still gets through:

<!-- listing: main.go#after: the agent's real date parser -->
```go
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
```
<!-- /listing -->

Read `defend` first. Three outcomes, and only one of them reaches the calendar writer. A date that does not exist comes back as `nil` and is rejected. A date that parsed but carries a warning makes `NeedsReview` true, and the event is held for a human instead of being written. Only a clean event is calendared. The small `integrity` helper drops one informational note the parser also returns, `assumed_year`, because these emails never state the year and assuming it is not a data problem; the product's own extractor drops it the same way.

## III. The same two lines, again

Now the second half of the same `go run .`, the exact same two inputs through the defended path:

```
== after: with the control ==
attack/back-to-school-night.txt
  email says:  Thursday, September 29th from 5:30PM-8PM
  parsed:      2026-09-29T17:30:00
  warnings:    weekday_mismatch: stated Thursday, but 2026-09-29 is Tuesday
  held for review, not calendared
attack/february-30.txt
  email says:  February 30th at 9:00AM
  rejected: no such date, nothing to calendar
attack blocked (the receipt)

== still works: a clean date ==
  email says:  Tuesday, September 29th 5:30PM - 8:00PM
  calendared:  DTSTART:20260929T173000  (2026-09-29 is a Tuesday)
```

The Thursday line still parses to September 29, because that is the only date in it, but now it arrives with a warning that says exactly what is off, and the event is held rather than written. February 30 does not become March 2; it becomes nothing. And the third block is the part people forget to check: the correct line from the same newsletter goes straight through to a `DTSTART`, so the control is not just refusing everything.

The last of the file is three small helpers that only exist to print the receipt: the `DTSTART` line out of the calendar text, that line back into the form the parser uses, and the real weekday of a date.

<!-- listing: main.go#small helpers for printing -->
```go
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
```
<!-- /listing -->

That is the whole program. The folder's tests say the same thing as `main`, and `make board` re-runs them after every later lesson so this line can never quietly go red again. Put this next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import "testing"

// The lesson's contract as tests, so the gate and the folder agree with the post.
// Both must pass before a lesson is done, and `make board` re-runs them after
// every later lesson. This lesson does not involve the model: the dates are
// decided by deterministic code on purpose, which is the whole point.

func TestAttackLandsUndefended(t *testing.T) {
	if !attackUndefended() {
		t.Fatal("the undefended path should calendar both wrong dates with no warning; otherwise there is nothing to fix")
	}
}

func TestAttackBlockedDefended(t *testing.T) {
	if !attackDefended() {
		t.Fatal("the defended path should hold the weekday mismatch for review and reject the impossible date")
	}
}

func TestCleanDateStillCalendared(t *testing.T) {
	if !cleanStillCalendared() {
		t.Fatal("the control must still calendar a correct date, or it is just refusing everything")
	}
}
```
<!-- /listing -->

```bash
go test -count=1 -v ./...
```

```
=== RUN   TestAttackLandsUndefended
--- PASS: TestAttackLandsUndefended (0.00s)
=== RUN   TestAttackBlockedDefended
--- PASS: TestAttackBlockedDefended (0.00s)
=== RUN   TestCleanDateStillCalendared
--- PASS: TestCleanDateStillCalendared (0.00s)
PASS
ok  	github.com/t0ul/ai-security-engineering/posts/01.2-dates-the-model-never-decides	0.260s
```

```
01.2   dates-the-model-never-decides      P1 ch1 parser            green
```

That first test is the one I would ask you to keep: it asserts that the naive parser really does calendar both wrong dates. If someone later "fixes" the undefended path, the test fails, because a lesson whose attack no longer lands is not teaching anything.

## Conclusion

So the date is decided twice over: once by code that will not normalize an impossible day, and once by a check that the stated weekday agrees with the date. Neither involves a model, which is the point, and both end in a warning or a refusal you can read, rather than a calendar entry you have to go looking for.

There is a loose end, on purpose. The Thursday event was "held for review, not calendared," but held where, and reviewed by whom? And even the clean event only became a file. Nothing in this agent adds anything to a real calendar on its own, and that is a decision, not a gap. The next post is about that file and the human step after it.

## Take it to your own project

Wherever a model hands you a fact that has consequences:

- Decide which facts are security-critical (dates, amounts, recipients) and never let the model be the authority on them.
- Extract those facts with deterministic code, and treat the model's version as a proposal to check.
- Reject values that cannot be true (an impossible date, a negative amount) outright.
- Flag internal contradictions for a human instead of guessing which side is right.
