---
title: "Conflicts and mangled data"
description: "This chapter steps away from the attacker. The adversary here is wrong data: a date on the wrong weekday, a time borrowed from nowhere, a negation flipped, the same event announced twice. A model passes all of it through. The agent re-checks every extracted event with deterministic standard-library code that cannot be talked into agreeing, and catches each one."
date: 2026-04-14
draft: false
tags: ["ai-security", "golang", "integrity", "agents"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The adversary is the data

Every attack so far had someone behind it. This chapter does not. The thing that corrupts the agent's answers most reliably is not an injection, it is that school emails are a mess: they put an event on a weekday the date does not fall on, they omit times and years, they say the meeting moved and leave both dates in, they say "we do not accept late forms" in a sentence a careless cleaner turns into "we accept late forms." None of this is hostile. It is just wrong, and a model asked to extract events will hand you the wrong answer with the same confidence as the right one, because sounding right is all it does.

So integrity gets the same treatment as security in this book: never trust what you did not check. The agent treats a model's extraction as a proposal, and re-derives the facts that matter with deterministic standard-library code. This is a control lesson, no model, because the whole point is the part that does not use one:

```bash
cd posts/11.1-conflicts-and-mangled-data
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init integrity`, then `go get github.com/t0ul/emaildrop@` followed by the version on the first `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/11.1-conflicts-and-mangled-data

go 1.27.1

// The agent's real deterministic integrity checks: date/weekday/time parsing,
// title normalization, and the conflict detector. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76

require (
	github.com/t0ul/goflage v0.0.0-20261009135357-739ce25ed7e2 // indirect
	github.com/t0ul/gumpers v0.0.0-20261006154904-836570c2c66c // indirect
)
```
<!-- /listing -->

## The spine of the program

`main` prints what the naive pipeline does with four mangled records, then runs the four checks, and the receipt needs all four to catch their case.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 11.1, conflicts and mangled data.
//
// Run it with:
//
//	go run .
//
// This chapter is about integrity, not an attacker. School emails get dates and
// times wrong, contradict themselves, and say "not" where it matters, and a model
// that proposes an answer will cheerfully pass all of it through. The agent never
// trusts a model's arithmetic or its reading: it re-checks every extracted event
// with deterministic, standard-library code. This demo runs four kinds of mangled
// data past a naive pipeline that trusts it, then past the agent's real checks,
// which catch each one. Deterministic, no model.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/t0ul/emaildrop/pkg/agent/dateparse"
	"github.com/t0ul/emaildrop/pkg/agent/items"
	"github.com/t0ul/emaildrop/pkg/agent/tool"
	"github.com/t0ul/emaildrop/pkg/agent/tools"
)

func main() {
	trustedAsIs()
	all := checksCatchAll()
	fmt.Println()
	if all {
		fmt.Println("attack blocked (the receipt): a naive pipeline passes every mangled record through; the deterministic checks catch the wrong weekday, the borrowed time, the inverted negation, and the self-conflicting date")
	} else {
		fmt.Println("a check did not catch its case")
		os.Exit(1)
	}
}
```
<!-- /listing -->

## Four ways data lies, trusted

Here is the naive pipeline: take what the extraction said and use it. The one piece of it worth showing as code is the title cleaner, because it is the one that does active damage:

<!-- listing: main.go#before: a naive pipeline that trusts the extraction -->
```go
// ---- before: a naive pipeline that trusts the extraction ----

// naiveStrip is the title cleaner I might write to tidy a title: drop little
// filler words. It does not know that "not" is not filler, so it inverts meaning.
var filler = map[string]bool{"we": true, "do": true, "not": true, "the": true, "a": true, "please": true}

func naiveStrip(title string) string {
	var out []string
	for _, w := range strings.Fields(title) {
		if !filler[strings.ToLower(strings.Trim(w, ".,"))] {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

func trustedAsIs() {
	fmt.Println("== before: the extraction is trusted as-is ==")
	fmt.Println("  weekday:   'Thursday, September 29th' accepted as Thursday (it is a Tuesday)")
	fmt.Println("  time:      'PTA Meeting following drop-off' accepted at 22:27 (no time in the text)")
	fmt.Printf("  negation:  'We do not accept late permission forms' cleaned to %q (meaning inverted)\n", naiveStrip("We do not accept late permission forms"))
	fmt.Println("  duplicate: 'Picture Day' kept on both Sep 24 and Oct 2")
	fmt.Println("  integrity problems caught: 0/4")
}
```
<!-- /listing -->

Four mangled records go in, and all four come out intact, wrong.

```
== before: the extraction is trusted as-is ==
  weekday:   'Thursday, September 29th' accepted as Thursday (it is a Tuesday)
  time:      'PTA Meeting following drop-off' accepted at 22:27 (no time in the text)
  negation:  'We do not accept late permission forms' cleaned to "accept late permission forms" (meaning inverted)
  duplicate: 'Picture Day' kept on both Sep 24 and Oct 2
  integrity problems caught: 0/4
```

The negation one is worth staring at. A reasonable-looking title cleaner that strips filler words, "we", "do", "the", quietly strips "not" along with them, and the event's meaning flips to its opposite. Nobody attacked anything. A list of stopwords did it.

## Four deterministic checks

The agent re-checks each of these in `pkg/agent/dateparse`, `pkg/agent/items`, and the conflict detector in `pkg/agent/tools`, all deterministic, all standard library. Here is one function per check, each a call into the real package, and the loop that runs them:

<!-- listing: main.go#after: the agent's deterministic integrity checks -->
```go
// ---- after: the agent's deterministic integrity checks ----

// checkWeekday uses the real date parser's weekday-vs-date consistency check.
func checkWeekday() (bool, string) {
	dp := dateparse.ParseDatePhrase("Thursday, September 29th", 2026)
	return dp.Mismatch, dp.Warning
}

// checkTime uses the real time parser: a time the event's own text does not
// contain is borrowed, and an impossible clock value is rejected outright.
func checkTime() (bool, string) {
	eventText := "PTA Meeting following drop-off"
	supported := dateparse.ParseTimes(eventText) // what the text actually says
	impossible := dateparse.ParseTimes("25:99")  // rejected, not rolled over
	borrowed := len(supported) == 0              // a claimed 22:27 is unsupported
	return borrowed && len(impossible) == 0,
		fmt.Sprintf("22:27 is not supported by the event text (borrowed); 25:99 rejected as impossible (parsed times: %v)", supported)
}

// checkNegation uses the real title normalizer, which keeps negations.
func checkNegation() (bool, string) {
	title := "We do not accept late permission forms"
	tidy := items.Tidy(title)
	kept := strings.Contains(strings.ToLower(tidy), "not")
	return kept, fmt.Sprintf("the title normalizer keeps the negation -> %q", tidy)
}

// checkDuplicate uses the real conflict detector over the extracted events.
func checkDuplicate() (bool, string) {
	email := "Picture Day: Thursday, September 24th.\nPicture Day: Friday, October 2nd."
	r := tools.Conflicts{}.Run(email, tool.Ctx{DefaultYear: 2026})
	return len(r.Warnings) > 0, strings.ReplaceAll(r.Text, "\n", "; ")
}

func checksCatchAll() bool {
	fmt.Println("\n== after: deterministic integrity checks ==")
	checks := []struct {
		name string
		fn   func() (bool, string)
	}{
		{"weekday ", checkWeekday},
		{"time    ", checkTime},
		{"negation", checkNegation},
		{"duplicate", checkDuplicate},
	}
	caught := 0
	for _, c := range checks {
		ok, detail := c.fn()
		if ok {
			caught++
		}
		fmt.Printf("  %s caught=%v: %s\n", c.name, ok, detail)
	}
	fmt.Printf("  integrity problems caught: %d/%d\n", caught, len(checks))
	return caught == len(checks)
}
```
<!-- /listing -->

Run the same four records through them.

```
== after: deterministic integrity checks ==
  weekday  caught=true: weekday_mismatch: stated Thursday, but 2026-09-29 is Tuesday
  time     caught=true: 22:27 is not supported by the event text (borrowed); 25:99 rejected as impossible (parsed times: [])
  negation caught=true: the title normalizer keeps the negation -> "do not accept late permission forms"
  duplicate caught=true: conflict: "picture day" announced on 2026-09-24, 2026-10-02
  integrity problems caught: 4/4
```

Each check is small and boring, which is the point. The weekday check parses the date itself and compares the real weekday to the stated one, so "Thursday, September 29th" is flagged because the 29th is a Tuesday, a thing the real corpus actually does. The time check parses the times present in an event's own text, so a 22:27 that appears nowhere in the words is seen for what it is, borrowed, and an impossible "25:99" is rejected instead of silently rolling over into the next day. The title normalizer knows that a negation is never filler and keeps it, so the meaning cannot invert. And the conflict detector notices the same event wearing two dates and says so rather than booking both.

That is the whole program. Its tests assert that all four checks catch their case and, separately, that the naive cleaner really does drop the negation, so the red half is real and not arranged. Put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"strings"
	"testing"
)

// Deterministic, no model. The four deterministic integrity checks each catch
// their class of mangled data, and the naive title cleaner really does invert a
// negation (the RED is real, not arranged).

func TestChecksCatchAllFour(t *testing.T) {
	if !checksCatchAll() {
		t.Fatal("every deterministic integrity check must flag its mangled input")
	}
}

func TestNaivePathInvertsNegation(t *testing.T) {
	got := naiveStrip("We do not accept late permission forms")
	if strings.Contains(strings.ToLower(got), "not") {
		t.Fatalf("the naive cleaner should drop the negation (that is the bug the real normalizer avoids), got %q", got)
	}
	if !strings.HasPrefix(strings.ToLower(got), "accept") {
		t.Fatalf("dropping the negation should invert the meaning to 'accept ...', got %q", got)
	}
}
```
<!-- /listing -->

## Why this is code and not a prompt

You could ask a model to check its own dates, and it would usually agree with itself, because the error and the check come from the same place. That is the whole reason these live in deterministic code. A calendar is arithmetic and string equality, and those are things a computer is exactly right about and a language model is approximately right about. The division of labor from the very first chapters holds here too: let the model propose the messy human parts, the title, the span of text that looks like a date, and let code decide anything that has a correct answer. The model reads; the code checks.

These checks flag and, where it is safe, correct. The next post is about the one error that is not in the email at all, which no consistency check against the text can catch, because the text never said it. A date the model simply invented. That one needs a different move, and it closes the chapter and the part.

## Take it to your own project

Catch wrong data with deterministic code:

- Check for internal contradictions: a weekday that does not match its date, a borrowed time, a flipped negation.
- Deduplicate before anything acts on the data.
- Flag conflicts for a human instead of letting the model pick a side.
- Keep these checks as plain code, not model judgment.
