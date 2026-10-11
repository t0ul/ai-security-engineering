---
title: "Dates the model never decides"
description: "The first field that bit me was the date. A naive parser happily calendars a Thursday that is really a Tuesday and a February 30 that does not exist, with no warning. This post shows that happening, then adds the agent's deterministic date parser, which flags the first and rejects the second, and proves it with a test you can run."
date: 2026-10-06
draft: true
tags: ["ai-security", "golang", "agents"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The field that bit me first

Of all the fields in [the event struct](/blog/one-shape-for-everything/), the one that caused trouble first was the date. School emails bury dates in prose, almost never say the year, and now and then they get the weekday wrong. That last one sounds harmless until you remember what the agent is for: a wrong date on the family calendar is the whole product failing, and failing quietly, because a calendar entry does not look wrong. It just sits there.

Security usually comes last. I would rather build it in as I go and prove it with a test, and this post is the first time that habit pays for itself, before a model is anywhere near the code. The whole lesson lives in one folder of the book repo, and it runs with nothing else set up:

```bash
cd posts/01.2-dates-the-model-never-decides
go run .
```

The folder pulls the agent's real date parser and calendar writer straight from the product repo at a pinned commit, so you do not need a checkout of emaildrop next to it. Let me walk through what it does.

## I. A parser that works

Here is the date parser I would have written first, and honestly did, in spirit. Find the month name and the day, find the first clock time, hand them to Go's `time.Date`, format the result:

```go
// naiveStart finds the month, the day, and the first clock time, and hands them
// to time.Date. It never looks at the weekday the email states, and time.Date
// quietly normalizes February 30 into March 2. This is the undefended path.
func naiveStart(when string) (string, bool) {
	m := reNaiveDate.FindStringSubmatch(when)
	if m == nil {
		return "", false
	}
	month := months[strings.ToLower(m[1])]
	day, _ := strconv.Atoi(m[2])
	hour, minute := 0, 0
	if t := reNaiveTime.FindStringSubmatch(when); t != nil {
		hour, _ = strconv.Atoi(t[1])
		if t[2] != "" {
			minute, _ = strconv.Atoi(t[2])
		}
		if strings.EqualFold(t[3], "PM") && hour < 12 {
			hour += 12
		}
		if strings.EqualFold(t[3], "AM") && hour == 12 {
			hour = 0
		}
	}
	d := time.Date(year, time.Month(month), day, hour, minute, 0, 0, time.UTC)
	return d.Format("2006-01-02T15:04:05"), true
}
```

(The two regular expressions just match a month name followed by a number, and a clock time with AM or PM. Nothing clever.) Give it a line like `Tuesday, September 29th 5:30PM - 8:00PM` and you get `2026-09-29T17:30:00`, which is right. The demo then builds an event with it and hands that event to the agent's real calendar writer, so what you see is the actual `DTSTART` line a calendar app would read.

Then I fed it two lines. The first is real: it is from one of the public sample newsletters in the product repo, `testdata/emaildrop/samples/6.txt`, exactly as it appears in that sample. The second I made up, because no school has sent me a February 30 yet, but a typo like it is only a matter of time. Both live in the lesson's `attack/` folder:

```
Back to School Night
Thursday, September 29th from 5:30PM-8PM
School Building
```

```
Winter Recital
February 30th at 9:00AM
Auditorium
```

Here is the first half of `go run .`:

```
== before: no defense ==
attack/back-to-school-night.txt
  email says:  Thursday, September 29th from 5:30PM-8PM
  calendared:  DTSTART:20260929T173000  (2026-09-29 is a Tuesday; warnings: 0)
attack/february-30.txt
  email says:  February 30th at 9:00AM
  calendared:  DTSTART:20260302T090000  (2026-03-02 is a Monday; warnings: 0)
attack landed (this is the problem the post fixes)
```

Read those two `calendared` lines slowly. The email said Thursday. The parser took September 29, which in 2026 is a Tuesday, and calendared it without a word, so now there is an entry that disagrees with the email it came from, and nobody knows which one is wrong. Then it took February 30 and calendared March 2. That second one is `time.Date` doing exactly what its documentation says: it normalizes, so the 30th of February becomes the 2nd of March, and the parser passes that along as if the email had said so.

Nobody attacked anything here. A school sent a sloppy line and the code did what it was told. That is the thing I want you to notice: the naive version does not fail, it succeeds at the wrong thing, and the only way you would ever find out is by missing Back to School Night. And once a model is the thing proposing the date text, a few posts from now, you will not be able to tell a typo from a nudge. So the fix has to live in code, below the model, and it has to be the same code every time.

## II. Dates the model never decides

The control is the agent's own date parser, `github.com/t0ul/emaildrop/pkg/agent/dateparse`. It is deterministic parsing plus integrity checking, written against the standard library and nothing else, and its one rule is in its name: the model is never asked to do date arithmetic. A model may later propose the span of text that holds the date, but turning that text into a real date, and checking that the date makes sense, happens here, in code I can read and test, the same way every run.

It does two things the naive parser did not. First, after calling `time.Date` it compares the result's year, month, and day back against what it was given, and if they moved, the date was impossible and the whole thing is rejected rather than normalized. Second, if the email states a weekday, it works out the real weekday of the date and compares the two, and a mismatch becomes a warning in plain words. Here is the lesson's defended path, which is just that parser plus the review rule from the last post:

```go
// defend runs one snippet through dateparse and decides what happens to it.
// It returns the .ics text when the event was calendared, or "" when it was
// rejected or held for review.
func defend(s snippet) (outcome string, calendared string) {
	r := dateparse.ExtractDatetime(s.When, year)
	if r == nil {
		return "rejected: no such date, nothing to calendar", ""
	}
	ev := schema.New(s.Title, r.Start)
	ev.End, ev.AllDay, ev.Location = r.End, r.AllDay, s.Where
	ev.Warnings = integrity(r.Warnings)
	if ev.NeedsReview() {
		return "held for review, not calendared", ""
	}
	text, _, err := ics.Write([]schema.Event{ev}, "lesson-01.2")
	if err != nil {
		panic(err)
	}
	return "calendared", text
}
```

Three outcomes, and only one of them reaches the calendar writer. A date that does not exist comes back as `nil` and is rejected. A date that parsed but carries a warning makes `NeedsReview` true, and the event is held for a human instead of being written. Only a clean event is calendared. The small `integrity` helper drops one informational note the parser also returns, `assumed_year`, because these emails never state the year and assuming it is not a data problem; the product's own extractor drops it the same way.

## III. The same two lines, again

Now the second half of the same `go run .`, the exact same two inputs through the defended path:

```
== after: with the control ==
attack/back-to-school-night.txt
  email says:  Thursday, September 29th from 5:30PM-8PM
  parsed:      2026-09-29T17:30:00
  warnings:    weekday_mismatch: stated Thursday, but 2026-09-29 is Tuesday
  held for review, not calendared
attack/february-30.txt
  email says:  February 30th at 9:00AM
  rejected: no such date, nothing to calendar
attack blocked (the receipt)

== still works: a clean date ==
  email says:  Tuesday, September 29th 5:30PM - 8:00PM
  calendared:  DTSTART:20260929T173000  (2026-09-29 is a Tuesday)
```

The Thursday line still parses to September 29, because that is the only date in it, but now it arrives with a warning that says exactly what is off, and the event is held rather than written. February 30 does not become March 2; it becomes nothing. And the third block is the part people forget to check: the correct line from the same newsletter goes straight through to a `DTSTART`, so the control is not just refusing everything.

The folder's tests say the same thing, and `make board` re-runs them after every later lesson so this line can never quietly go red again:

```bash
go test -count=1 -v ./...
```

```
=== RUN   TestAttackLandsUndefended
--- PASS: TestAttackLandsUndefended (0.00s)
=== RUN   TestAttackBlockedDefended
--- PASS: TestAttackBlockedDefended (0.00s)
=== RUN   TestCleanDateStillCalendared
--- PASS: TestCleanDateStillCalendared (0.00s)
PASS
ok  	github.com/t0ul/ai-security-engineering/posts/01.2-dates-the-model-never-decides	0.260s
```

```
01.2   dates-the-model-never-decides      P1 ch1 parser            green
```

That first test is the one I would ask you to keep: it asserts that the naive parser really does calendar both wrong dates. If someone later "fixes" the undefended path, the test fails, because a lesson whose attack no longer lands is not teaching anything.

## Conclusion

So the date is decided twice over: once by code that will not normalize an impossible day, and once by a check that the stated weekday agrees with the date. Neither involves a model, which is the point, and both end in a warning or a refusal you can read, rather than a calendar entry you have to go looking for.

There is a loose end, on purpose. The Thursday event was "held for review, not calendared," but held where, and reviewed by whom? And even the clean event only became a file. Nothing in this agent adds anything to a real calendar on its own, and that is a decision, not a gap. The next post is about that file and the human step after it.
