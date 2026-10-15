// Command main is the runnable demo for lesson 13.2, tasks and reminders.
//
// Run it with:
//
//	go run .
//
// The agent now pulls more than meetings out of an email: to-dos, heads-ups, and
// actions, each tagged with a kind. An action is the dangerous kind, because it
// carries a URL to act on, so it must be surfaced for review, not dropped silently
// into a to-do list. The natural way to classify, by what the sentence sounds
// like, lets an attacker frame a phishing link as a chore and hide it. The agent's
// real classifier checks for a URL first, so a link is always an action. No model.
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/t0ul/emaildrop/pkg/agent/items"
	"github.com/t0ul/emaildrop/pkg/agent/schema"
)

func main() {
	undef := hiddenUndefended()
	def := surfacedDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): to-do framing hides a link in the task list on the naive classifier; the real classifier checks for a URL first, so the link is always a surfaced action, not a silent chore")
	} else {
		fmt.Println("the classifier did not behave as described")
		os.Exit(1)
	}
}

// ---- the email with a smuggled link ----

const phishURL = "http://attacker.example/phish"

// email has a genuine reminder (no link) and a smuggled action: a phishing link
// dressed up as a routine to-do, "don't forget to submit ... at <url>".
const email = "PTA News\n\n" +
	"Reminder: please return the signed field-trip form by Friday.\n\n" +
	"Also, don't forget to submit the permission form at " + phishURL + " before Monday.\n"

// ---- before: classify by what the sentence sounds like ----

var reTaskWords = regexp.MustCompile(`(?i)\b(reminder|don't forget|do not forget|submit|return|deadline|due|rsvp|sign up|register)\b`)
var reLink = regexp.MustCompile(`(?i)\bhttps?://\S+`)

// naiveClassify files anything that sounds like a to-do as a task, and only calls
// something an action if it did not already look like a chore. A link wrapped in
// to-do language therefore becomes a silent task, its URL never surfaced.
func naiveClassify(text string) []schema.Event {
	var out []schema.Event
	for _, s := range strings.Split(text, "\n") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		switch {
		case reTaskWords.MatchString(s):
			out = append(out, schema.Event{Title: s, Kind: schema.KindTask})
		case reLink.MatchString(s):
			out = append(out, schema.Event{Title: s, Kind: schema.KindAction, URL: reLink.FindString(s)})
		}
	}
	return out
}

// ---- the oracle: the item carrying the link ----

// linkItem returns the classified item whose text carries the phishing link.
func linkItem(items []schema.Event) (schema.Event, bool) {
	for _, e := range items {
		if strings.Contains(e.Title, phishURL) || e.URL == phishURL {
			return e, true
		}
	}
	return schema.Event{}, false
}

// ---- the attack, against the naive classifier ----

func hiddenUndefended() bool {
	fmt.Println("== before: a classifier that files to-dos first ==")
	out := naiveClassify(email)
	for _, e := range out {
		fmt.Printf("  kind=%-7s url=%-30q %s\n", e.ResolvedKind(), e.URL, trunc(e.Title))
	}
	it, ok := linkItem(out)
	hidden := ok && it.ResolvedKind() == schema.KindTask && it.URL == ""
	fmt.Printf("  the phishing link is filed as a chore, no action flag, URL not surfaced: %v\n", hidden)
	return hidden
}

// ---- after: the agent's real classifier checks for a link first ----

func surfacedDefended() bool {
	fmt.Println("\n== after: the agent's real classifier (URL-first) ==")
	out := items.Classify(email, 2026)
	for _, e := range out {
		fmt.Printf("  kind=%-7s url=%-30q %s\n", e.ResolvedKind(), e.URL, trunc(e.Title))
	}
	it, ok := linkItem(out)
	surfaced := ok && it.ResolvedKind() == schema.KindAction && it.URL == phishURL
	// and the genuine reminder is still a task
	var genuineTask bool
	for _, e := range out {
		if strings.Contains(e.Title, "field-trip") && e.ResolvedKind() == schema.KindTask {
			genuineTask = true
		}
	}
	fmt.Printf("  the link is an action with its URL surfaced for review: %v (genuine reminder still a task: %v)\n", surfaced, genuineTask)
	return surfaced && genuineTask
}

// ---- helpers ----

func trunc(s string) string {
	if len(s) > 52 {
		return s[:52] + "..."
	}
	return s
}
