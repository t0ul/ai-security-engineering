---
title: "Inert output and the human accept"
description: "The safest thing an email agent can do with an event is write it to a file and stop. This post shows why the one-line shortcut of adding events straight to your calendar is the bug, and builds the two-part control that replaces it: output that changes nothing on its own, and a human accept step that only adds what a person chose."
date: 2025-09-16
draft: false
tags: ["ai-security", "golang", "agents"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The one-line shortcut

At the end of [the date lesson](/blog/dates-the-model-never-decides/) I held a flagged event back for review and left two questions hanging: held where, and reviewed by whom. This post answers them, and it starts with the shortcut I was tempted to take instead.

The agent has extracted some events. The obvious next line is to put them on the calendar. In a toy build that is one loop, and it demos beautifully: drop an email in, watch events appear on your schedule. So it is worth being clear about why I did not write it, because the reason is the whole shape of this project.

The lesson runs on its own from the book repo, no model involved:

```bash
cd posts/01.3-inert-output-and-the-human-accept
go run .
```

It carries two events forward from the date lesson. One is clean: a Multicultural Potluck on Friday, October 23, where the weekday matches the date. The other is the flagged one: an Open House stated as Thursday, September 29, which in 2026 is a Tuesday, so the parser marked it for review.

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init accept`, then `go get github.com/t0ul/emaildrop@` followed by the version on the `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/01.3-inert-output-and-the-human-accept

go 1.27.1

// The agent's real calendar writer and event shape, pulled from the product repo
// at the commit this lesson was written against. No local overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
```
<!-- /listing -->

## The spine of the program

The top of the file carries the two events, a `calendar`, and `main`. `extracted` runs the two lines through the agent's real date parser with the same integrity rule as the last lesson, so the Open House arrives already flagged. The `calendar` is a stand-in for the real thing: in the agent it is your actual calendar app, the one that will buzz your phone at the wrong time; here it is a list we can inspect, so "did a bad event reach the calendar" has a yes-or-no answer. Then `main` runs the shortcut, the control, and one check that the control still lets a good event through, and prints the receipt only if all three came out the way this post says.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 01.3, inert output and the human
// accept.
//
// Run it with:
//
//	go run .
//
// Before: the agent writes straight to a real calendar. A flagged extraction (an
// event whose stated weekday does not match its date) lands on the calendar with
// no human in the loop.
//
// After: the agent only ever writes an inert .ics file, which changes no real
// calendar by itself, and a human accept step adds only the events a person chose.
// The flagged event is not in the safe default a one-click accept would add, so it
// stays off the calendar until someone deliberately imports it.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/t0ul/emaildrop/pkg/agent/dateparse"
	"github.com/t0ul/emaildrop/pkg/agent/ics"
	"github.com/t0ul/emaildrop/pkg/agent/schema"
)

const year = 2026

// extracted is the two events as the parser returns them: one clean, one flagged.
// See attack/README.md.
func extracted() []schema.Event {
	return []schema.Event{
		parse("Multicultural Potluck", "Friday October 23rd 5:30PM-7:00PM", "School Cafeteria"),
		parse("Open House", "Thursday September 29th 5:30PM-8:00PM", "School Building"),
	}
}

// parse runs one line through the agent's real date parser and keeps the integrity
// warning (dropping the informational assumed_year note, as the product does), so
// a weekday mismatch makes the event need review, exactly as in the date lesson.
func parse(title, when, where string) schema.Event {
	r := dateparse.ExtractDatetime(when, year)
	e := schema.New(title, r.Start)
	e.End, e.AllDay, e.Location = r.End, r.AllDay, where
	for _, w := range r.Warnings {
		if !strings.HasPrefix(w, "assumed_year") {
			e.Warnings = append(e.Warnings, w)
		}
	}
	return e
}

// calendar stands in for a real calendar: the thing an event lands on and starts
// reminding you about. In the agent this is your actual calendar app; here it is a
// list we can inspect, so "did a bad event reach the calendar" is a question with a
// yes or no answer.
type calendar struct{ entries []string }

func (c *calendar) add(e schema.Event) { c.entries = append(c.entries, e.Title+" @ "+e.Start) }
func (c *calendar) has(title string) bool {
	for _, s := range c.entries {
		if strings.HasPrefix(s, title+" @") {
			return true
		}
	}
	return false
}

func main() {
	fmt.Println("== before: no defense ==")
	if attackUndefended() {
		fmt.Println("attack landed (this is the problem the post fixes)")
	} else {
		fmt.Println("the undefended path should have put the flagged event on the calendar")
		os.Exit(1)
	}

	fmt.Println("\n== after: with the control ==")
	if attackDefended() {
		fmt.Println("attack blocked (the receipt)")
	} else {
		fmt.Println("the defended path should have kept the flagged event off the calendar")
		os.Exit(1)
	}

	fmt.Println("\n== still works: a human accepts the clean event ==")
	if !cleanEventAccepted() {
		fmt.Println("the accept step should still let a human add a good event")
		os.Exit(1)
	}
}
```
<!-- /listing -->

## I. Straight to the calendar

Here is the shortcut. Take the extracted events and add them. Under it is the undefended run, which does exactly that and then asks the calendar whether the flagged event is on it:

<!-- listing: main.go#before: write straight to the calendar -->
```go
// ---- before: write straight to the calendar ----

// autoAdd is the shortcut I was tempted to write: take the extracted events and put
// them on the calendar. It is one line, it demos beautifully, and it is the bug,
// because it moves an unreviewed extraction onto your real schedule with nothing in
// between.
func autoAdd(cal *calendar, events []schema.Event) {
	for _, e := range events {
		cal.add(e)
	}
}

// attackUndefended auto-adds both events and reports whether the flagged one
// reached the calendar. It did, which is the attack landing: a parse the agent
// itself marked for review is now on your schedule, and you were never asked.
func attackUndefended() bool {
	cal := &calendar{}
	events := extracted()
	autoAdd(cal, events)
	for _, e := range events {
		flag := ""
		if e.NeedsReview() {
			flag = "  <- flagged for review, added anyway"
		}
		fmt.Printf("  calendar += %s @ %s%s\n", e.Title, e.Start, flag)
	}
	return cal.has("Open House")
}
```
<!-- /listing -->

Run the first half:

```
== before: no defense ==
  calendar += Multicultural Potluck @ 2026-10-23T17:30:00
  calendar += Open House @ 2026-09-29T17:30:00  <- flagged for review, added anyway
attack landed (this is the problem the post fixes)
```

The agent flagged the Open House itself, one lesson ago, and then put it on the calendar anyway, because `autoAdd` never looks at the flag. Nobody attacked anything. The code did the direct thing, and the direct thing writes an unreviewed parse onto your real schedule with no stop in between.

Hold that next to where this series is going. Soon a language model, not a regex, proposes these events, after reading an email a stranger wrote. If the only gap between "the model said so" and "it is on your calendar" is one `for` loop, then every mistake and every planted instruction the model follows lands on your schedule automatically. The fix is not a smarter parser. It is to remove the automatic path entirely.

## II. Output that changes nothing

So the agent's output is a file, and only a file. It writes an `.ics`, the standard calendar format from the setup post, and stops. The piece that does this is the product's own writer, `github.com/t0ul/emaildrop/pkg/agent/ics`. It renders a list of events to iCalendar text and nothing more: it opens no calendar, makes no network call, and runs nothing. The file sits in the outbox until a person opens it. That inertness is the point, and it is why I reach for a writer that returns text rather than one that talks to a calendar API.

Here is the whole defended path: the safe default a one-click accept offers, the accept itself, the run that writes the file and then applies that default, and the check that the clean event still gets through. Read `attackDefended` top to bottom against the two output blocks that follow.

<!-- listing: main.go#after: an inert file plus a human accept -->
```go
// ---- after: an inert file plus a human accept ----

// safeDefault is what a one-click "accept" offers: the events that are not flagged
// for review. A flagged event is left unchecked, so adding it takes a deliberate
// second action by someone who has seen the warning.
func safeDefault(events []schema.Event) map[int]bool {
	approved := map[int]bool{}
	for i, e := range events {
		if !e.NeedsReview() {
			approved[i] = true
		}
	}
	return approved
}

// accept adds to the calendar only the events whose index a human approved. Nothing
// here reaches the calendar without an entry in approved; that map is the gate.
func accept(cal *calendar, events []schema.Event, approved map[int]bool) {
	for i, e := range events {
		if approved[i] {
			cal.add(e)
		}
	}
}

// attackDefended runs the defended flow: write the events to an inert .ics file,
// confirm that writing the file changed no calendar, then apply the safe default a
// human would be offered. It returns true when the flagged event never reached the
// calendar.
func attackDefended() bool {
	cal := &calendar{}
	events := extracted()

	// The agent's only output is a file. ics.Write is the product's real writer; it
	// renders an inert calendar file and touches nothing live.
	text, _, err := ics.Write(events, "email-to-calendar")
	if err != nil {
		panic(err)
	}
	fmt.Printf("  wrote an inert .ics (%d bytes); calendar entries now: %d\n", len(text), len(cal.entries))
	fmt.Println("  the file is just text until a person imports it, e.g.:")
	for _, line := range firstLines(text, 6) {
		fmt.Printf("    %s\n", line)
	}

	// A person reviews. The safe default offers only the clean event; the flagged
	// one is unchecked, with its warning shown.
	approved := safeDefault(events)
	for i, e := range events {
		state := "accept"
		if !approved[i] {
			state = "hold: " + strings.Join(e.Warnings, "; ")
		}
		fmt.Printf("  [%s] %s @ %s\n", state, e.Title, e.Start)
	}
	accept(cal, events, approved)
	fmt.Printf("  calendar after accept: %v\n", cal.entries)
	return !cal.has("Open House")
}

// cleanEventAccepted confirms the gate is not just refusing everything: the clean
// event, accepted in the safe default, really does reach the calendar.
func cleanEventAccepted() bool {
	cal := &calendar{}
	events := extracted()
	accept(cal, events, safeDefault(events))
	fmt.Printf("  calendar: %v\n", cal.entries)
	return cal.has("Multicultural Potluck")
}
```
<!-- /listing -->

The first half of the defended run writes that file:

```
== after: with the control ==
  wrote an inert .ics (833 bytes); calendar entries now: 0
  the file is just text until a person imports it, e.g.:
    BEGIN:VCALENDAR
    VERSION:2.0
    PRODID:-//ai-security-engineering//email-to-calendar//EN
    X-WR-CALNAME:email-to-calendar
    BEGIN:VEVENT
    UID:f5ed71d77c293f61635e2f3e14046697@email-to-calendar
```

Both events, the clean one and the flagged one, went into that text. The line that matters is the count: after writing the file, the calendar has zero entries. Writing the output changed nothing live. (The `UID` is a random id per file, so yours will differ; everything else is the same.) An `.ics` is inert by nature. Your calendar app imports it only when you double-click it, which means there is now a person standing between the extraction and the schedule. That is the gap `autoAdd` did not have.

## III. The accept, and who does it

A file nobody ever imports is safe and useless. The second half of the control is the accept step: the one place an event crosses from the file onto the calendar, and it is driven by a person, deny-by-default. That is `accept`, above: it adds to the calendar only the events whose index a human approved, and the `approved` map is the gate. Nothing is approved unless a person approved it. The question is what a person is shown, and here the review flag from the last lesson earns its keep. A one-click accept offers a safe default: the events that are not flagged for review are pre-checked, and a flagged event is left unchecked, so adding it takes a deliberate second action by someone who has read the warning.

```
  [accept] Multicultural Potluck @ 2026-10-23T17:30:00
  [hold: weekday_mismatch: stated Thursday, but 2026-09-29 is Tuesday] Open House @ 2026-09-29T17:30:00
  calendar after accept: [Multicultural Potluck @ 2026-10-23T17:30:00]
attack blocked (the receipt)
```

The clean Potluck was in the safe default and went on. The Open House was held, with its warning shown next to it, and never reached the calendar. The same bad extraction that `autoAdd` calendared without a word is now sitting in front of a person who can see exactly what is wrong with it. That is the answer to the two questions from the date lesson: held in the file, reviewed by you.

And the control is not just refusing everything. The last block confirms the clean event still goes through when a person accepts the default:

```
== still works: a human accepts the clean event ==
  calendar: [Multicultural Potluck @ 2026-10-23T17:30:00]
```

The one helper at the bottom of the file only trims the calendar text for printing:

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

func firstLines(s string, n int) []string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return lines
}
```
<!-- /listing -->

That is the whole program. The folder's tests assert all three of its checks: the shortcut calendars the flagged event, the inert-file-plus-accept path keeps it off, and a human accepting the default still adds the clean one. Put this next to `main.go` as `contract_test.go` and run `go test`; `make board` reruns it after every later lesson, so this control cannot quietly rot.

<!-- listing: contract_test.go#all -->
```go
package main

import "testing"

// The lesson's contract as tests, so the gate and the folder agree with the post.
// This lesson is deterministic: no model decides whether an event reaches the
// calendar. The accept gate does, and these assert it.

func TestAttackLandsUndefended(t *testing.T) {
	if !attackUndefended() {
		t.Fatal("auto-add should put the flagged event on the calendar; otherwise there is nothing to fix")
	}
}

func TestAttackBlockedDefended(t *testing.T) {
	if !attackDefended() {
		t.Fatal("the inert-file-plus-accept path should keep the flagged event off the calendar")
	}
}

func TestCleanEventAccepted(t *testing.T) {
	if !cleanEventAccepted() {
		t.Fatal("a human accepting the safe default must still add the clean event")
	}
}
```
<!-- /listing -->

## Conclusion

Two small decisions, and neither needs a model. The agent's output is inert, so writing it changes nothing on its own. And a real calendar gains an event only through an accept a person drives, with anything suspect left unchecked. Between them there is no automatic path from an extraction to your schedule, which is the property that will still hold three parts from now when the thing proposing the events is a model reading hostile mail.

It is worth saying what this is not. This is the plain version of a human-in-the-loop gate, good enough for a parser you trust. Once the agent can take actions that are harder to undo than a calendar entry, the accept needs more: evidence shown up front, a single-use confirmation, resistance to a forged click. That is its own lesson later in the book. For now the agent has a dependable core: one shape, honest dates, inert output, a human gate. Next I give it the last piece of that core, the part that watches a folder and picks up the mail, and I show what goes wrong when the thing it reads is hostile or simply too big.

## Take it to your own project

Before your agent can change anything real:

- Make its output inert by default: a file, a draft, a proposal that nothing applies on its own.
- Put an explicit human accept step between the proposal and any irreversible effect.
- Separate "propose" from "apply" in the code, so the apply path cannot run without a person.
- Make the accept decision specific: approve this item, not a blanket "trust the agent."
