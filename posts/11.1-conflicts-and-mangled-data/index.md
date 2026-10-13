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

## Four ways data lies, trusted

Here is the naive pipeline: take what the extraction said and use it. Four mangled records go in, and all four come out intact, wrong.

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

The agent re-checks each of these in `pkg/agent/dateparse`, `pkg/agent/items`, and the conflict detector in `pkg/agent/tools`, all deterministic, all standard library. Run the same four records through them.

```
== after: deterministic integrity checks ==
  weekday  caught=true: weekday_mismatch: stated Thursday, but 2026-09-29 is Tuesday
  time     caught=true: 22:27 is not supported by the event text (borrowed); 25:99 rejected as impossible (parsed times: [])
  negation caught=true: the title normalizer keeps the negation -> "do not accept late permission forms"
  duplicate caught=true: conflict: "picture day" announced on 2026-09-24, 2026-10-02
  integrity problems caught: 4/4
```

Each check is small and boring, which is the point. The weekday check parses the date itself and compares the real weekday to the stated one, so "Thursday, September 29th" is flagged because the 29th is a Tuesday, a thing the real corpus actually does. The time check parses the times present in an event's own text, so a 22:27 that appears nowhere in the words is seen for what it is, borrowed, and an impossible "25:99" is rejected instead of silently rolling over into the next day. The title normalizer knows that a negation is never filler and keeps it, so the meaning cannot invert. And the conflict detector notices the same event wearing two dates and says so rather than booking both.

## Why this is code and not a prompt

You could ask a model to check its own dates, and it would usually agree with itself, because the error and the check come from the same place. That is the whole reason these live in deterministic code. A calendar is arithmetic and string equality, and those are things a computer is exactly right about and a language model is approximately right about. The division of labor from the very first chapters holds here too: let the model propose the messy human parts, the title, the span of text that looks like a date, and let code decide anything that has a correct answer. The model reads; the code checks.

These checks flag and, where it is safe, correct. The next post is about the one error that is not in the email at all, which no consistency check against the text can catch, because the text never said it. A date the model simply invented. That one needs a different move, and it closes the chapter and the part.

## Take it to your own project

Catch wrong data with deterministic code:

- Check for internal contradictions: a weekday that does not match its date, a borrowed time, a flipped negation.
- Deduplicate before anything acts on the data.
- Flag conflicts for a human instead of letting the model pick a side.
- Keep these checks as plain code, not model judgment.
