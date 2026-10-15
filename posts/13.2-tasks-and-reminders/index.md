---
title: "Tasks and reminders"
description: "The agent now pulls to-dos, heads-ups, and actions out of an email, not just meetings. An action is the dangerous kind, because it carries a URL to act on, so it has to be surfaced for review rather than dropped into a silent to-do list. Classify by what a sentence sounds like and an attacker hides a phishing link behind to-do language. The real classifier checks for a URL first, so a link is always an action."
date: 2026-06-09
draft: false
tags: ["ai-security", "golang", "agents", "classification"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## More than meetings

The capstone chapter builds the agent out to the thing you would actually use, one feature at a time, each arriving with its own attack and its own fix. First feature: an email does not only announce events. It also carries to-dos ("return the signed form"), heads-ups ("guest author visiting"), and actions ("RSVP at this link"). So the agent learns to pull all of them out, tagging each item with a kind: event, task, heads-up, or action.

That last kind is the one with teeth. An action carries a URL to act on, and a URL in an email you did not vet is exactly the thing the whole book has been careful about. A task is a note to yourself; an action is a door. The agent treats them differently: an action is surfaced for you to look at before you touch the link, while a task can sit quietly on a list. That difference is the feature, and it is also the vulnerability, because it means an attacker wins by getting a link filed as a task. A control lesson, deterministic, no model:

```bash
cd posts/13.2-tasks-and-reminders
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init items`, then `go get github.com/t0ul/emaildrop@` followed by the version on the first `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/13.2-tasks-and-reminders

go 1.27.1

// The agent's real item classifier (events, tasks, heads-ups, actions). No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76

require github.com/t0ul/goflage v0.0.0-20261009135357-739ce25ed7e2 // indirect
```
<!-- /listing -->

## The spine of the program

`main` classifies the same email two ways, and the receipt needs the link hidden as a task the first time and surfaced as an action the second.

<!-- listing: main.go#head -->
```go
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
```
<!-- /listing -->

## Classify by what it sounds like, and lose

Here is the email. One genuine reminder with no link, and one phishing link dressed up as a chore.

<!-- listing: main.go#the email with a smuggled link -->
```go
// ---- the email with a smuggled link ----

const phishURL = "http://attacker.example/phish"

// email has a genuine reminder (no link) and a smuggled action: a phishing link
// dressed up as a routine to-do, "don't forget to submit ... at <url>".
const email = "PTA News\n\n" +
	"Reminder: please return the signed field-trip form by Friday.\n\n" +
	"Also, don't forget to submit the permission form at " + phishURL + " before Monday.\n"
```
<!-- /listing -->

The obvious way to classify is by what each sentence sounds like: if it reads like a to-do, "reminder", "don't forget", "submit", it is a task. That rule is reasonable, common, and wrong, because the second sentence reads exactly like a to-do, and it has a link in it.

<!-- listing: main.go#before: classify by what the sentence sounds like -->
```go
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
```
<!-- /listing -->

The oracle finds the item carrying the link, whatever kind it was given, and the undefended run asks whether that item ended up a task with no URL surfaced:

<!-- listing: main.go#the oracle: the item carrying the link -->
```go
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
```
<!-- /listing -->

<!-- listing: main.go#the attack, against the naive classifier -->
```go
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
```
<!-- /listing -->

```
== before: a classifier that files to-dos first ==
  kind=task    url=""                             Reminder: please return the signed field-trip form b...
  kind=task    url=""                             Also, don't forget to submit the permission form at ...
  the phishing link is filed as a chore, no action flag, URL not surfaced: true
```

Both land as tasks. The attacker's link is now sitting in your to-do list, indistinguishable from returning a form, its URL not surfaced for review because the agent decided this was a chore and chores do not have links worth showing. The owner clears their list, clicks to "submit the form", and lands on the attacker.

## Check for the link first

The fix is an ordering, and it is one line of judgment: a URL decides the kind before any to-do vocabulary gets a vote. The agent's real classifier in `pkg/agent/items` checks for a link first, and only if there is none does it ask whether the sentence sounds like a task or a heads-up. This is the switch from the product repo, `pkg/agent/items/items.go`, quoted because the lesson imports the package:

```go
switch {
case hasURL:  ev.Kind = schema.KindAction; ev.URL = reURL.FindString(s)
case action:  ev.Kind = schema.KindTask
default:      ev.Kind = schema.KindHeadsUp
}
```

Run the same email through it, with the one extra check that the genuine reminder is still a task:

<!-- listing: main.go#after: the agent's real classifier checks for a link first -->
```go
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
```
<!-- /listing -->

What it prints:

```
== after: the agent's real classifier (URL-first) ==
  kind=task    url=""                             Return the signed field-trip form by
  kind=action  url="http://attacker.example/phish" Don't forget to submit the permission form at http:/...
  the link is an action with its URL surfaced for review: true (genuine reminder still a task: true)
```

The genuine reminder is still a task, because it has no link. The smuggled one is now an action, with its URL pulled out into its own field where the owner, and the human gate from the agency chapter, will see it before anything acts on it. The to-do language did not help the attacker at all, because the classifier never asked what the sentence sounded like until after it had already found the link.

The one helper trims a title for printing:

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

func trunc(s string) string {
	if len(s) > 52 {
		return s[:52] + "..."
	}
	return s
}
```
<!-- /listing -->

That is the whole program. Its tests assert both halves and one more case, a single sentence that is both a chore and a link. Put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/agent/items"
	"github.com/t0ul/emaildrop/pkg/agent/schema"
)

// Deterministic, no model. A phishing link wrapped in to-do language is filed as a
// silent task by a sounds-like classifier, and surfaced as an action with its URL
// by the agent's real, URL-first classifier. The link is a benign marker.

func TestSmuggledLinkHiddenUndefended(t *testing.T) {
	if !hiddenUndefended() {
		t.Fatal("the naive classifier must file the URL-bearing item as a task with no surfaced URL; otherwise there is nothing to fix")
	}
}

func TestRealClassifierSurfacesAction(t *testing.T) {
	if !surfacedDefended() {
		t.Fatal("the real classifier must tag the link as an action with its URL and keep the genuine reminder a task")
	}
}

func TestURLAlwaysWinsOverTaskWords(t *testing.T) {
	// A single sentence that is BOTH a chore and a link is an action, never a task.
	out := items.Classify("Reminder: submit the form at "+phishURL+" by Friday.", 2026)
	it, ok := linkItem(out)
	if !ok || it.ResolvedKind() != schema.KindAction || it.URL != phishURL {
		t.Fatalf("a URL-bearing chore must classify as an action with its URL, got %+v", out)
	}
}
```
<!-- /listing -->

## The cheap rule beats the clever one

It would be tempting to make this smarter, to score how suspicious a link looks or train something to spot phishing. The agent does the boring thing instead: a link is an action, full stop, no matter how friendly the sentence around it. That is the same move as never letting the model decide a date, or never running a tool beyond its capability. The security-relevant decision, is this a thing that acts, is made by a rule that cannot be social-engineered, and the model-shaped judgment is only allowed to run on what is left. A phishing link cannot talk its way into the task list, because the thing that sorts it never listens to it.

The next feature is the weekly view, pulling all of this together into "what is my week", and its attack is quieter: not a link, but a crafted item that tries to make one real day look like another.

## Take it to your own project

When you add a feature, re-apply the controls:

- Give a new item type the same inert output, human accept, and validation as the first.
- Do not let a new surface skip the controls the old ones have.
- Attack the new feature before you trust it.
