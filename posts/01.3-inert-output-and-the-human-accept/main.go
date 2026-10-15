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

// ---- helpers ----

func firstLines(s string, n int) []string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return lines
}
