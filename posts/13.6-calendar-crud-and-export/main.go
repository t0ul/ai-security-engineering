// Command main is the runnable demo for lesson 13.6, calendar CRUD and export.
//
// Run it with:
//
//	go run .
//
// The danger here is not an attacker. It is the agent doing the same safe thing
// twice: a watcher re-reads an email, a request is retried, an export runs again,
// and the same event gets written to the calendar a second time. A naive write
// fires on every call, so the calendar fills with duplicates. The agent routes
// each write through a durable exactly-once ledger keyed by the event's content,
// so a replay returns the first result and never writes again. Deterministic.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/t0ul/emaildrop/pkg/agent/schema"
	"github.com/t0ul/emaildrop/pkg/durable"
)

// calendar is the side the write touches: the household's actual calendar. A
// duplicate here is a duplicate on their phone.
var calendar []string

func resetCalendar() { calendar = nil }

// addEvent is the side effect: it writes one event onto the calendar.
func addEvent(title string) (string, error) {
	calendar = append(calendar, title)
	return title, nil
}

// the same email, extracted into the same event, processed twice (a re-ingest).
func events() []schema.Event {
	e := schema.Event{Title: "Multicultural Potluck", Start: "2026-10-23", Kind: schema.KindEvent}
	return []schema.Event{e}
}

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// eventKey is the content fingerprint that makes the write idempotent: the same
// event, however many times it is re-ingested, maps to the same key.
func eventKey(e schema.Event) string {
	norm := strings.Trim(nonAlnum.ReplaceAllString(strings.ToLower(e.Title), "-"), "-")
	return norm + "|" + e.Start + "|" + e.ResolvedKind()
}

// ---- before: write on every pass ----

func importNaive(evs []schema.Event) {
	for _, e := range evs {
		_, _ = addEvent(e.Title)
	}
}

func duplicatesUndefended() bool {
	fmt.Println("== before: a write that fires on every pass ==")
	resetCalendar()
	importNaive(events()) // first ingest
	importNaive(events()) // the watcher re-reads the same email
	dup := len(calendar) == 2
	fmt.Printf("  calendar after two ingests of one email: %v\n", calendar)
	fmt.Printf("  duplicate written: %v\n", dup)
	return dup
}

// ---- after: the durable exactly-once ledger ----

func importDurable(l *durable.Log, evs []schema.Event) (replays int) {
	for _, e := range evs {
		_, replayed, _ := l.Do("import", "calendar-write", eventKey(e), func() (string, error) {
			return addEvent(e.Title)
		})
		if replayed {
			replays++
		}
	}
	return
}

func exactlyOnceDefended() bool {
	fmt.Println("\n== after: the durable exactly-once ledger ==")
	resetCalendar()
	l, _ := durable.Open("") // in-memory ledger
	importDurable(l, events())
	replays := importDurable(l, events()) // the retry: deduped by key
	once := len(calendar) == 1
	fmt.Printf("  calendar after two ingests of one email: %v\n", calendar)
	fmt.Printf("  written exactly once: %v (the replay was recognized and skipped: %v)\n", once, replays == 1)
	return once && replays == 1
}

func main() {
	undef := duplicatesUndefended()
	def := exactlyOnceDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("the receipt: a naive write duplicates the event when the email is re-ingested; the durable ledger keys the write by the event's content, so the replay returns the first result and the calendar holds exactly one")
	} else {
		fmt.Println("the write did not behave exactly once")
		os.Exit(1)
	}
}
