---
title: "One shape for everything"
description: "Before the parser, before the model, the first thing I wrote for the agent was a single typed record for a calendar event. This post shows that struct, runs it, and shows every stage of the agent agreeing on it, so that nothing a model says ever gets to be the data."
date: 2026-10-05
draft: true
tags: ["ai-security", "golang", "agents"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## Back to an empty project

As promised at the end of [the last post](/blog/attack-driven-design-shown-once/), we rewind. No model, no defenses, nothing to attack yet. Part 1 is about building the boring core well enough that attacking it later actually means something.

So here is the very first thing I wrote, before a parser, before a watcher, before a single line that reads an email. It is a struct. (If you are new to Go, a struct is just a named bundle of fields, a record with a fixed shape.) I wrote it on October 5 as a Python dataclass, eight fields with a comment each, and when I started porting the agent to Go the next day, the first file in the new tree was this same shape, sitting next to the date parser.

That order is not an accident. The agent is a pipeline: something reads the email, something pulls events out of it, something checks the dates, something writes the calendar file, something scores the result, and later a web page shows it all. Several of those stages will touch text I did not write and, soon, the output of a model that read that text. If every stage agrees on one typed record, then "what the model said" is never the data. The data is this struct, and only code gets to fill it in. That is a small decision, but every later post leans on it, so I want you to see it plainly before the stakes go up.

## I. The struct

From your emaildrop checkout, ask Go to show it to you:

```bash
go doc ./pkg/agent/schema Event
```

```
type Event struct {
	Title       string   `json:"title"`
	Start       string   `json:"start"`
	End         string   `json:"end,omitempty"`
	AllDay      bool     `json:"all_day"`
	Location    string   `json:"location,omitempty"`
	SourceEmail string   `json:"source_email,omitempty"`
	Confidence  float64  `json:"confidence"`         // 0..1; low-confidence events get flagged for review
	Warnings    []string `json:"warnings,omitempty"` // e.g. "weekday_mismatch: stated Thu, 2026-09-29 is Tue"

	Kind  string `json:"kind,omitempty"`  // "" = event; see Kind* constants
	Due   string `json:"due,omitempty"`   // task/action due date (distinct from Start)
	Notes string `json:"notes,omitempty"` // the asking line / short free text
	URL   string `json:"url,omitempty"`   // action only; validated + egress-gated before any click
	Recur string `json:"recur,omitempty"` // iCalendar RRULE for a recurring event, e.g. "FREQ=WEEKLY;BYDAY=TU"
}
    Event is one extracted calendar item. Start/End/Due are ISO 8601
    ("2026-09-24T08:30:00", or "2026-09-24" when AllDay).

func New(title, start string) Event
func (e Event) NeedsReview() bool
func (e Event) ResolvedKind() string
```

Let me walk the first eight, since those are the original shape. `Title` is the event's name. `Start` and `End` are dates written as ISO 8601 strings, which is the international date format: year, month, day, a `T`, then the time, so `2026-09-24T08:30:00`. When `AllDay` is true the string is just the day. `Location` is where. `SourceEmail` is which file this came from, which matters the moment you have to trace an event back to the email that produced it. `Confidence` is a number from 0 to 1 saying how sure the extractor was. And `Warnings` is a list of plain strings describing anything odd. The example in the comment, an email that says Thursday but gives a date that falls on a Tuesday, is the whole of the next post.

The five fields after the blank line (`Kind`, `Due`, `Notes`, `URL`, `Recur`) arrived a day or two later, when the agent grew up to handle tasks, reminders, and links. You can ignore them for now. They are here because this is the real struct from the finished project, and I would rather show you the real thing with a few fields you do not need yet than a tidied-up fiction.

Two small things are worth noticing. The dates are strings, not Go's `time.Time`. That looks lazy until you see that the same string goes into the JSON on disk, gets compared by the eval, and gets formatted by the calendar writer: one representation, readable in a log line, and it never changes meaning crossing a boundary. And the tags in backticks fix the name every field has when it is written out as JSON, so a file written today reads the same next month. The `omitempty` on some of them means an empty field is left out of the output entirely rather than written as blank.

The helpers are small. `New` builds an event with the confidence defaulted to 1.0 and an empty (but not missing) warnings list, the way the Python dataclass did. `NeedsReview` is true when the confidence is below 0.75 or there is any warning at all. (`ResolvedKind` only matters for the later fields: it treats an empty kind as a plain event.) A couple of posts from now, that one method decides what a human has to look at before anything reaches a calendar.

## II. What it looks like on the wire

Reading a struct is one thing. Let us run it. Make a throwaway folder in your emaildrop checkout (do not commit it) and put this program in it:

```bash
mkdir -p cmd/shape
```

```go
// cmd/shape/main.go
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/t0ul/ai-security-engineering/pkg/agent/extractor"
	"github.com/t0ul/ai-security-engineering/pkg/agent/schema"
)

func main() {
	// 1. the constructor: what a brand-new event looks like on the wire
	ev := schema.New("PTA Meeting", "2026-09-24T08:30:00")
	ev.Location = "Gymnatorium"
	b, _ := json.MarshalIndent(ev, "", "  ")
	fmt.Println("== schema.New ==")
	fmt.Println(string(b))
	fmt.Println("needs review:", ev.NeedsReview())

	// 2. the regex extractor on a real sample: every event it returns is this one shape
	text, err := os.ReadFile("testdata/emaildrop/samples/3.txt")
	if err != nil {
		panic(err)
	}
	events := extractor.ExtractEvents(string(text), "3.txt", 2026)
	fmt.Printf("\n== extractor.ExtractEvents(samples/3.txt): %d events ==\n", len(events))
	for _, e := range events {
		b, _ := json.Marshal(e)
		fmt.Println(string(b))
	}

	// 3. JSON with keys the shape has no slot for: they fall away on decode
	raw := `{"title":"Book fair","start":"2026-10-02","all_day":true,"confidence":1,"command":"rm -rf /","sneaky_url":"http://evil.example"}`
	var decoded schema.Event
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		panic(err)
	}
	b, _ = json.Marshal(decoded)
	fmt.Println("\n== decode JSON with extra keys ==")
	fmt.Println("in: ", raw)
	fmt.Println("out:", string(b))
}
```

Run it from the root of the checkout, so the sample path resolves:

```bash
go run ./cmd/shape
```

```
== schema.New ==
{
  "title": "PTA Meeting",
  "start": "2026-09-24T08:30:00",
  "all_day": false,
  "location": "Gymnatorium",
  "confidence": 1
}
needs review: false

== extractor.ExtractEvents(samples/3.txt): 3 events ==
{"title":"PTA Meeting","start":"2026-09-24T08:30:00","all_day":false,"location":"Gymnatorium","source_email":"3.txt","confidence":0.9}
{"title":"Back-to-School Night","start":"2026-09-29T17:30:00","end":"2026-09-29T20:00:00","all_day":false,"location":"School Building","source_email":"3.txt","confidence":0.9}
{"title":"Multicultural Potluck","start":"2026-10-23T17:30:00","end":"2026-10-23T19:00:00","all_day":false,"location":"School Cafeteria","source_email":"3.txt","confidence":0.9}

== decode JSON with extra keys ==
in:  {"title":"Book fair","start":"2026-10-02","all_day":true,"confidence":1,"command":"rm -rf /","sneaky_url":"http://evil.example"}
out: {"title":"Book fair","start":"2026-10-02","all_day":true,"confidence":1}
```

Three things happened there, and each one is a small promise the rest of the book cashes in.

First, the constructor. You see `omitempty` doing its job: `end`, `source_email`, and `warnings` were never set, so they are simply not in the output. `all_day` and `confidence` always print, because a reader of this file should never have to guess whether they were left out or false. And `needs review: false`, because the confidence is 1.0 and there are no warnings.

Second, the regex extractor on the same newsletter from the setup post hands back exactly three events, the three Save the Dates that the eval harness scored at `f1=1.00`. (The full agent run found more because it also pulls out tasks and days off; this is only the event extractor.) Every one of them is the same shape, and notice the confidence: 0.9, not the constructor's 1.0. The extractor sets its own number, and because 0.9 is above the 0.75 threshold, none of these three would be held for a human.

Third, the decode. I handed it a JSON object with two keys the struct has no slot for, and Go's `encoding/json` kept the four fields it knows and dropped the rest without complaint. I want to be careful about what that is and is not. It is not a defense, and in the agent the model's answer never even decodes into an `Event` (next section). But it does make the shape a ceiling. There is no `command` field, so there is no way for a command to ride along inside an event, however it got into the JSON. Nothing downstream can carry a field the struct does not name.

## III. Every stage agrees

The reason to have one shape is that everyone uses it. Here is how you check that claim rather than take it from me. From the checkout:

```bash
grep -rl 'schema\.Event' pkg | grep -v _test | sort
```

```
pkg/agent/ensemble/ensemble.go
pkg/agent/eval/eval.go
pkg/agent/extractor/extractor.go
pkg/agent/ics/ics.go
pkg/agent/items/items.go
pkg/agent/pipeline/pipeline.go
pkg/agent/tool/tool.go
pkg/domain/domain.go
pkg/redteam/platform.go
pkg/server/confirm.go
pkg/server/events.go
pkg/server/items.go
pkg/server/webapp.go
```

Thirteen files, and between them that is the whole agent. The extractor produces events, and both of its paths, the rules and the model, return the same `[]schema.Event`, so nothing downstream can tell which one an event came from. The ensemble package is the reconciler from the last post: it takes the rules' list and the model's list and compares them, and it can only do that because both sides are the same shape. The `ics` package turns a list of them into the calendar file. The `eval` package scores a list of them against the hand-written answers. The pipeline carries them from the inbox to the outbox, and the `items`, `server`, and `domain` packages are the tasks and the web page that come much later. One type, no translation layers, no second shape for the UI.

Now the part I most want you to take away. The model never produces an `Event`. In `pkg/agent/extractor/extractor.go`, the model's answer is decoded into this, and only this:

```go
var arr []struct {
	Title string `json:"title"`
	When  string `json:"when"`
	Where string `json:"where"`
}
```

Three strings. That is the whole of what the model gets to say about an event: a name, a phrase like "Thursday, September 24 at 8:30", and a place. Everything else, the actual start date, whether it is all day, the confidence, the warnings, is decided by ordinary code that reads those strings and that I can test. The model proposes; the struct is filled in by code. That is the shape of every control in this book, honestly, just at its smallest.

## Conclusion

So that is the shape. Thirteen fields, three small helpers, no dependencies, and every stage of the agent speaks it. It is the most boring code in the project, and it is the reason the rest of the project is testable: when a later post says "the gala never made it out," what that means is that no `schema.Event` with that date left the reconciler, and that is a sentence you can turn into a test.

Next we take the field that causes the most trouble, the date. The next post builds the date parser and shows the two cases that bit me: an email that says Thursday but gives a Tuesday's date, and a date that does not exist at all. A naive parse puts both on the calendar. The fix flags them instead.
