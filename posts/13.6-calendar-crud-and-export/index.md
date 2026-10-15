---
title: "Calendar CRUD and export"
description: "The danger in this feature is not an attacker. It is the agent doing the same safe thing twice: a watcher re-reads an email, a request is retried, an export runs again, and the same event lands on the calendar a second time. A naive write fires on every call. The agent routes each write through a durable exactly-once ledger keyed by the event's content, so a replay returns the first result and never writes again."
date: 2026-07-07
draft: false
tags: ["ai-security", "golang", "agents", "reliability"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The quiet failure

Most of this book has been about a hostile email. This feature fails without one. Writing events to the calendar and exporting them is the agent's one durable side effect, the thing that shows up on your actual phone, and the way it goes wrong is the most ordinary bug there is: it happens twice. The folder watcher restarts and re-reads an email it already processed. A request times out and the client retries. An export runs a second time. Every one of those is correct behavior, a safe operation repeated, and every one of them, done naively, puts a second copy of the same event on your calendar.

It is not a security attack in the usual sense, but it is the same discipline, because a side effect that fires more than once is exactly how a retried refund becomes a double refund. Getting "exactly once" right is a reliability property and a safety property at the same time. A control lesson, deterministic, no model:

```bash
cd posts/13.6-calendar-crud-and-export
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init once`, then `go get github.com/t0ul/emaildrop@` followed by the version on the `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/13.6-calendar-crud-and-export

go 1.27.1

// The agent's real durable exactly-once ledger and event shape. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
```
<!-- /listing -->

## The spine of the program

`main` ingests the same email twice through a naive write and twice through the ledger, and the receipt needs two copies the first time and one the second.

<!-- listing: main.go#head -->
```go
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
```
<!-- /listing -->

The calendar is the side the write touches, a slice standing in for the household's phone; `addEvent` is the side effect; `events` is the one event the email yields; and `eventKey` is the content fingerprint that will make the write idempotent, the same event mapping to the same key however many times it is re-ingested.

<!-- listing: main.go#the calendar, and the key for an event -->
```go
// ---- the calendar, and the key for an event ----

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
```
<!-- /listing -->

## Write on every pass, duplicate on every pass

Here is the naive write: an event comes in, put it on the calendar. Under it, the same email processed twice, the way a restarted watcher would.

<!-- listing: main.go#before: write on every pass -->
```go
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
```
<!-- /listing -->

```
== before: a write that fires on every pass ==
  calendar after two ingests of one email: [Multicultural Potluck Multicultural Potluck]
  duplicate written: true
```

Two Potlucks. The agent did nothing wrong on either pass; each time it saw the event and added it, which is what a write does. The duplicate is not a bug in the write, it is the absence of any memory that this particular write already happened.

## Key the write, remember it happened

That memory is the fix, and it has a standard shape: idempotency. Give each write a key derived from the event's content, the title, the date, the kind, so the same event always produces the same key no matter how many times it is re-ingested. Then route the write through a durable ledger that records which keys it has already done. The agent uses `pkg/durable` for this, the same exactly-once ledger that keeps a retried action from firing its side effect twice.

<!-- listing: main.go#after: the durable exactly-once ledger -->
```go
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
```
<!-- /listing -->

Process the same email twice through it.

```
== after: the durable exactly-once ledger ==
  calendar after two ingests of one email: [Multicultural Potluck]
  written exactly once: true (the replay was recognized and skipped: true)
```

One Potluck. The first pass ran the write and recorded its key; the second pass looked the key up, saw it was already done, and returned the first result instead of writing again. The watcher can re-read that email a hundred times and the calendar does not change after the first, because the ledger turns "do this" into "do this if it has not been done", keyed by what the thing actually is.

That is the whole program. Its tests assert both halves and that a replay returns the first call's cached result; put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/durable"
)

// Deterministic, no model. Re-ingesting one email writes the event twice on a
// naive write, and exactly once through the durable ledger, which recognizes the
// replay by the event's content key. No attacker; the hazard is a safe retry.

func TestReingestDuplicatesUndefended(t *testing.T) {
	if !duplicatesUndefended() {
		t.Fatal("a naive write must duplicate on re-ingest; otherwise there is nothing to make idempotent")
	}
}

func TestDurableLedgerWritesExactlyOnce(t *testing.T) {
	if !exactlyOnceDefended() {
		t.Fatal("the durable ledger must write the event exactly once and recognize the replay")
	}
}

func TestReplayReturnsCachedResult(t *testing.T) {
	resetCalendar()
	l, _ := durable.Open("")
	e := events()[0]
	out1, replayed1, _ := l.Do("import", "calendar-write", eventKey(e), func() (string, error) { return addEvent(e.Title) })
	out2, replayed2, _ := l.Do("import", "calendar-write", eventKey(e), func() (string, error) { return addEvent(e.Title) })
	if replayed1 || !replayed2 {
		t.Fatalf("first call should run (replayed=false), second should be a replay (replayed=true); got %v, %v", replayed1, replayed2)
	}
	if out1 != out2 {
		t.Fatalf("the replay must return the first call's cached result, got %q vs %q", out1, out2)
	}
	if len(calendar) != 1 {
		t.Fatalf("the side effect must fire exactly once, calendar has %d", len(calendar))
	}
}
```
<!-- /listing -->

## Exactly once is a safety property

It is worth ending the feature on why this belongs in a security book at all. The export is dup-free for the same reason: it keys each event and skips the ones already exported, so importing into a real calendar happens exactly once. And the ledger underneath is the same primitive that will matter far more in the reliability chapters, where the side effect is not adding a calendar line but spending money or sending a message. "At most once" for a dangerous action is the same engineering as "exactly once" for a calendar write: a durable record of what has already happened, keyed so a replay cannot pretend to be new. The agent is allowed to retry anything; it is not allowed to let a retry count twice.

That is the last of the agent's own features. The one surface left in the capstone is the one people touch directly, the chat box over everything the agent knows, and its attack brings the whole book full circle: a prompt injection, arriving this time through the corpus the chat retrieves.

## Take it to your own project

Guard the writes, not just the reads:

- Put least privilege and the human gate on writes and deletes.
- Sanitize exported fields and keep the export inert.
- Attack create, update, delete, and export each on its own.
