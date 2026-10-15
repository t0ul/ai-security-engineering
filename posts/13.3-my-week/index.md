---
title: "My week at a glance"
description: "The weekly view groups the household's items by day, and which day an item lands on is a decision an email can lie about: state one weekday while the date says another. A view that places items by the weekday written in the text files them on the wrong day and leaves the real day looking empty. The agent places by the date it parsed, derives the weekday from that, and flags the mismatch."
date: 2026-06-16
draft: false
tags: ["ai-security", "golang", "agents", "integrity"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The view that lays out your week

The last feature sorted items by kind. This one lays them out in time: My Week, the seven-day strip that answers "what have we got going on" at a glance. It is the view people actually open, and building it means a new decision the agent has to make for every item, which day does this land on.

That decision looks trivial and is not, because an email gets a vote in it. Dates in school emails are written for humans, with the weekday spelled out next to the number, "Friday, October 2nd", and a weekday is redundant information that can disagree with the date it sits beside. Usually that is an honest typo. It can also be crafted. A control lesson, deterministic, no model:

```bash
cd posts/13.3-my-week
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init week`, then `go get github.com/t0ul/emaildrop@` followed by the version on the `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/13.3-my-week

go 1.27.1

// The agent's real deterministic date/weekday parser. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
```
<!-- /listing -->

## The spine of the program

`main` sorts the items so the output is stable, then lays the week out both ways. The receipt needs the crafted item on the wrong day the first time and on its real day, flagged, the second.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 13.3, my week at a glance.
//
// Run it with:
//
//	go run .
//
// The weekly view groups the household's items by day. Which day an item lands on
// is a placement decision, and an email can lie about it: state one weekday while
// the date says another ("Monday ... October 8th" when the 8th is a Thursday). A
// view that places items by the weekday written in the text puts the item on the
// wrong day. The agent places by the date it parsed, derives the weekday from that
// date, and flags the mismatch. Deterministic, no model.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/t0ul/emaildrop/pkg/agent/dateparse"
)

func main() {
	sort.Strings(items) // stable output
	undef := misplacedUndefended()
	def := placedByDateDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): a stated weekday moves the item to the wrong day on the naive view; the agent places by the date it parsed, so the item lands on its real day and the lie is flagged")
	} else {
		fmt.Println("the view did not place as described")
		os.Exit(1)
	}
}
```
<!-- /listing -->

## Place by what the email says, and land on the wrong day

Here is the week. A genuine Book Fair, a genuine PTA meeting, and one crafted item: a sync that says Monday but is dated October 8th, which is a Thursday.

<!-- listing: main.go#the items, and the crafted one -->
```go
// ---- the items, and the crafted one ----

// The week's items. One genuine, and one crafted to claim a weekday its date does
// not fall on, so a naive view will file it on the wrong day.
var items = []string{
	"Book Fair on Friday, October 2nd",
	"PTA Meeting on Thursday, October 8th",
	"Mandatory all-staff sync on Monday, October 8th", // the lie: Oct 8 is a Thursday
}

const crafted = "Mandatory all-staff sync on Monday, October 8th"
```
<!-- /listing -->

The natural way to build the view is to read the weekday off each item and drop it in that column. It is right there in the text, after all.

<!-- listing: main.go#before: place by the weekday the email says -->
```go
// ---- before: place by the weekday the email says ----

// naiveWeek files each item under the weekday written in its text. The crafted
// item says Monday, so it lands on Monday, nowhere near the Thursday it is on.
func naiveWeek() map[string][]string {
	week := map[string][]string{}
	for _, it := range items {
		dp := dateparse.ParseDatePhrase(it, 2026)
		if dp == nil || dp.StatedWeekday == "" {
			continue
		}
		week[dp.StatedWeekday] = append(week[dp.StatedWeekday], it)
	}
	return week
}
```
<!-- /listing -->

```
== before: place items by the weekday the email states ==
  Monday:   Mandatory all-staff sync
  Thursday: PTA Meeting
  Friday:   Book Fair
  crafted item on Monday: true | on its real day (Thursday): false
```

The sync is on Monday now. Not on Thursday, where it actually falls, but on Monday, because that is the word the email used. A parent scanning their week sees a quiet Thursday and a Monday commitment that is not real, plans around both, and is wrong twice from one crafted line. Nobody clicked a link; the attack is just a date that lies about its own weekday, and a view that believed it.

## Place by the date, derive the day

The fix is the rule from the integrity chapter, now doing a job in the UI: the date is the authority, the weekday is decoration. The agent's My Week groups items by the date it parsed and derives the weekday from that date, so the word in the text never decides a column. And because it already knows both, it can say when they disagree. Here is that placement, with the three small functions that read a day back and print the week:

<!-- listing: main.go#after: place by the date, derive the day from it -->
```go
// ---- after: place by the date, derive the day from it ----

// realWeek files each item under the weekday of the date it actually falls on, the
// way the agent's My Week does (it groups by the parsed date), and flags any item
// whose stated weekday disagrees.
func realWeek() (map[string][]string, []string) {
	week := map[string][]string{}
	var flags []string
	for _, it := range items {
		dp := dateparse.ParseDatePhrase(it, 2026)
		if dp == nil {
			continue
		}
		week[dp.ActualWeekday] = append(week[dp.ActualWeekday], it)
		if dp.Mismatch {
			flags = append(flags, dp.Warning)
		}
	}
	return week, flags
}

func on(week map[string][]string, day string) []string { return week[day] }

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func printWeek(week map[string][]string) {
	order := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	for _, d := range order {
		if len(week[d]) == 0 {
			continue
		}
		short := make([]string, len(week[d]))
		for i, t := range week[d] {
			short[i] = strings.SplitN(t, " on ", 2)[0]
		}
		fmt.Printf("  %-9s %s\n", d+":", strings.Join(short, "; "))
	}
}
```
<!-- /listing -->

The two runs `main` calls ask the same two questions of each placement, where the crafted item landed and where it did not:

<!-- listing: main.go#the attack, against both placements -->
```go
// ---- the attack, against both placements ----

func misplacedUndefended() bool {
	fmt.Println("== before: place items by the weekday the email states ==")
	w := naiveWeek()
	printWeek(w)
	// The crafted item shows up on Monday, and is missing from its real day, Thursday.
	onMonday := contains(on(w, "Monday"), crafted)
	onThursday := contains(on(w, "Thursday"), crafted)
	fmt.Printf("  crafted item on Monday: %v | on its real day (Thursday): %v\n", onMonday, onThursday)
	return onMonday && !onThursday
}

func placedByDateDefended() bool {
	fmt.Println("\n== after: place items by the date, weekday derived from it ==")
	w, flags := realWeek()
	printWeek(w)
	onThursday := contains(on(w, "Thursday"), crafted)
	onMonday := contains(on(w, "Monday"), crafted)
	flagged := false
	for _, f := range flags {
		if strings.Contains(f, "stated Monday") {
			flagged = true
		}
	}
	fmt.Printf("  crafted item on its real day (Thursday): %v | on Monday: %v\n", onThursday, onMonday)
	for _, f := range flags {
		fmt.Printf("  flagged: %s\n", f)
	}
	return onThursday && !onMonday && flagged
}
```
<!-- /listing -->

The defended run:

```
== after: place items by the date, weekday derived from it ==
  Thursday: Mandatory all-staff sync; PTA Meeting
  Friday:   Book Fair
  crafted item on its real day (Thursday): true | on Monday: false
  flagged: weekday_mismatch: stated Monday, but 2026-10-08 is Thursday
```

The sync is on Thursday, next to the PTA meeting it shares the day with, and Monday is empty because nothing is on it. The owner sees the real shape of the week. And the mismatch is not swallowed silently, it is surfaced: stated Monday, but the 8th is a Thursday, the same deterministic check from the conflicts post, reused to label a crafted item instead of just correcting it. Honest placement, and a note about why the email was wrong.

That is the whole program. Its two tests are the two runs; put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import "testing"

// Deterministic, no model. A crafted item that states a weekday its date does not
// fall on lands on the wrong day in a view that trusts the text, and on its real
// day (with the mismatch flagged) in the agent's date-first view.

func TestStatedWeekdayMisplacesUndefended(t *testing.T) {
	if !misplacedUndefended() {
		t.Fatal("placing by the stated weekday must put the crafted item on the wrong day; otherwise there is nothing to fix")
	}
}

func TestDateFirstPlacesCorrectlyAndFlags(t *testing.T) {
	if !placedByDateDefended() {
		t.Fatal("placing by the parsed date must put the item on its real day and flag the weekday mismatch")
	}
}
```
<!-- /listing -->

## The same authority, everywhere it matters

There is nothing new in this fix, and that is the point worth making at the capstone. The weekday check was built chapters ago to catch a mangled date in extraction; here it places an item in a calendar view; the same code will decide which day a reminder fires on and which day a question about "this week" is answered for. One deterministic authority on what day a date is, used everywhere a day matters, so there is no second place for a crafted weekday to slip through. The model and the email can suggest; the date, parsed once by code, decides.

The next feature opens the agent up to people, a contact directory built from who appears in your mail, and its attack is about disclosure: surfacing the right contacts to the owner without quietly building a dossier an injected email can ask for.

## Take it to your own project

A convenience view is still a security surface:

- Scope any aggregated view to the asker.
- Render retrieved content inertly in the view.
- Never let a digest leak across tenants.
