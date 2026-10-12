---
title: "Hallucinated dates"
description: "The last post caught data that was wrong in the email. This one catches data that was never in the email: a date the model invented for an event the email left undated. No check against the text can catch it, because the text is silent. The agent reconciles the model's proposals against the dates the deterministic parser actually found, and drops any that are not there. It closes the integrity chapter and Part 5."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "integrity", "hallucination", "camel"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## A date that was never there

The last post caught data that was wrong in the email: a weekday that did not match its date, a borrowed time, a flipped negation. Every one of those was catchable because the email contradicted itself, and a deterministic check could see the contradiction. This post is about the error with no contradiction to find. The email simply does not say when an event is, and the model, asked for a date, makes one up.

The very first time I showed the Attack-Driven Design loop, back in [that post](/blog/attack-driven-design-shown-once/), this was the example, shown once and then set aside. Here is the full treatment. It is a control lesson, and because the error is something only a live model produces, it measures the real model:

```bash
cd posts/11.2-hallucinated-dates
go run .
```

## The model fills the gap

The email lists two events: Picture Day on October 2nd, and a Winter Gala with no date at all. Hand it to the local model with a request for structured events, and it does something very human. It does not leave the date blank. It puts down a confident, specific, invented date.

```
  model proposed:   Picture Day(2026-10-02), Winter Gala(2026-12-31)
  off-text dates:   [Winter Gala@2026-12-31]
```

December 31st appears nowhere in the email. The model produced it because a blank felt wrong, and a model's whole job is to produce the plausible next token, not to admit it does not know. If you trust the model's dates, that invented one goes straight onto the calendar, indistinguishable from the real one, and you find out on December 31st.

```
  trust the model's dates (keep every proposal):   100%
```

Every run. The gap gets filled every time, because filling gaps is the behavior, not the exception.

## Reconcile against the text

No check against the email catches this, because the email never made a claim to disagree with. The only authority is the set of dates that are actually written in it. That is what `pkg/agent/ensemble` enforces. It runs two extractors, the deterministic parser and the model, and reconciles them with one security-critical rule: a model-proposed event whose date does not appear in the raw email is a hallucination, and it is dropped. The deterministic dates are the authority; the model is a quarantined proposer whose output has to be grounded in the source before it counts.

```go
if d := dateKey(le); d != "" && !dates[d] {
    continue // hallucinated date: not in the email, so reject
}
```

Run the same extraction through it:

```
  after reconcile:  Picture Day(2026-10-02)
  off-text dates:   []

  reconcile against dates found in the text:        0%
```

The Winter Gala is gone. Not corrected, not guessed at, dropped, because the honest answer to "when is the gala" is that the email does not say, and an agent that invents the answer is worse than one that admits the gap. An event both extractors found, on a date the text contains, is kept and boosted; a model-only event on a real date is kept but flagged for review; a model-only event on an invented date is refused. You can watch the whole thing yourself: the live-tagged test makes the model hallucinate, then fails if a single invented date survives reconciliation.

```bash
go test -tags live -v ./...
```

This is the same shape as the dual-LLM split from Part 4, pointed at a different problem. There the model read untrusted text and could not act; here the model proposes facts and cannot assert one the source does not support. In both, the model is useful exactly up to the line where it would be trusted, and deterministic code holds that line.

## The board, at the end of Part 5

Part 5 gave the agent memory and then an honest relationship with it: retrieval that cannot be poisoned or leaked across people, and data it refuses to get wrong or make up. With the integrity chapter closed, the agent is feature-complete for everything the first five parts set out to build, so the whole book re-attacks itself one more time.

```
00.0  introduction                        prose
00.1  the-plan                            prose
00.2  setting-up-the-lab                  prose
00.3  attack-driven-design-shown-once     prose
01.1  one-shape-for-everything            prose
01.2  dates-the-model-never-decides       green
01.3  inert-output-and-the-human-accept   green
01.4  the-watched-folder                  green
02.1  the-eval-harness                    prose
02.2  attacking-the-baseline              prose
03.1  a-log-you-can-trust                 green
03.2  a-gateway-you-cannot-route-around   green
03.3  one-trace-id                        prose
04.1  when-the-email-gives-orders         green
04.2  the-email-is-data-not-orders        green
04.3  system-prompt-leakage               green
05.1  scrubbing-secrets-and-pii           green
05.2  the-calendar-field-exfil            green
05.3  the-tracking-pixel                  green
05.4  a-search-tool-hits-a-local-server   green
06.1  least-privilege                     green
06.2  the-human-gate                      green
06.3  circuit-breakers                    green
07.1  the-problem-with-running-code       prose
07.2  the-detonation-chamber              prose
07.3  egress-deny-and-the-broker          green
07.4  no-shell                            green
08.1  the-dual-llm-brain                  green
08.2  agent-to-agent-hand-off             green
09.1  pin-the-tools                       green
09.2  model-artifacts                     prose
09.3  defeating-the-lethal-trifecta       green
10.1  answering-whats-on-this-month       prose
10.2  index-poisoning                     green
10.3  cross-tenant-leak                   green
10.4  memory-poisoning                    green
11.1  conflicts-and-mangled-data          green
11.2  hallucinated-dates                  green

green 26  red 0  prose 12
```

Twenty-six runnable controls holding, twelve prose lessons, no red. The parts so far built the agent and made it safe one attack at a time. Part 6 changes the question. Instead of adding another control, it asks how you keep all of this true as the agent grows: one command that scores the whole thing, the attacks turned into a reusable library, a red-team that runs continuously, and the same method pointed at an app that is not this one. The controls are the work; the flywheel is how the work stays done.
