---
title: "A live red-team against the real model"
description: "The live model suite has been in this book's gate since the first attack. This post makes the case for it directly. A suite that skips the model when it is slow, or swaps in a stub, reports green while testing nothing. The honest gate fires the real attack at the real model, reports a measured number, and fails closed when the model is gone. The live number here is 100 percent: the raw model obeys the injection every time."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "testing", "red-team"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The suite that tests nothing

Every live-tagged test in this book has had the same comment at the top: run against the local model, fail if it is down, never skip. This post is why. A security suite for an LLM agent has a tempting shortcut built into every testing framework, and taking it quietly turns the whole scorecard into decoration.

The shortcut is to not run the hard part. The model is the slow, stateful, annoying dependency, so the two most natural things to do with it in a test are to skip it when it is not up, or to replace it with a stub that returns something predictable. Both make the suite fast and green. Both also make it a liar. This is a control lesson, and its deterministic halves run anywhere; the live number needs the local model:

```bash
cd posts/12.4-live-red-team
go run .
```

## Two ways to be green and wrong

First the stub. Swap a fake in for the model, one that always refuses, and the attack can never succeed, so the row is green. It stays green if you delete every real defense, because there was never a real model to defend. It is a test of the stub, not the agent.

```
== a stub swapped in for the model: permanently green ==
  attack vs the stub: marker returned=false -> ASR 0% (proves nothing)
```

Then the skip. The honest-looking version: if the model is down, skip the test. The suite reports a pass, a green check, a clean push, and it ran nothing at all.

```
== the model is down ==
  naive gate (skips):  pass=true  (skipped: model down (reported PASS))
  honest gate (fails): pass=false  err=model unreachable: the gate fails, it does not skip
```

Look at the two lines together. Same situation, model unreachable, and the naive gate calls it a pass while the honest gate calls it a failure. The honest one is right. A red-team that cannot reach its target has not found the target safe, it has not tested it, and the only safe thing to report is red. Fail closed, never skip, is the same fail-closed discipline from the control chapters applied to the test suite itself.

## Fire the real attack

With the model up, the honest gate does the thing the other two avoided: it fires a real injection at the real model and reads what comes back.

```
== the model is up: the same attack, fired for real ==
  injection vs the raw model: ASR 100% (measured live, this run)
```

One hundred percent. Told plainly to ignore its instructions and echo a marker, the raw model does it every single time. That number is the whole argument. The danger the rest of this book defends against is not hypothetical and it is not rare, it is the default behavior of the model, measurable in four requests that take about a second. A suite that stubbed or skipped its way past this would be reporting green over a system that obeys an attacker on the first try.

This is also why the live suite is cheap enough to never need the shortcut. The excuse for skipping, that a real model is too slow for a gate, does not survive contact with a local model that boots in seconds and answers in milliseconds. The attacks in this book run live on every push because they can, and because the one time the model is down is exactly the time you want the gate to stop you, not wave you through.

## The flywheel turns on reality

That closes the loop the chapter has been building. The one command is only a gate because its rows are wired; the library makes every row the same honest shape; the judge that scores subjective outputs cannot be talked into a pass; and the whole thing runs against the actual model, every time, or fails. None of it depends on remembering to be careful. It depends on the suite refusing to lie, even to make itself green.

There is one claim left to test, the biggest one. Everything so far has been this agent, secured by its own method. The next post points the exact same method, the gorauder catalog, a technique, the ADD gate, at an app that is not this one, and runs its live suite green, to find out whether any of this was ever really about email at all.
