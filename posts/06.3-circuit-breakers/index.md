---
title: "Circuit breakers and runaway loops"
description: "The last abuse in this part needs no approval and no overreach: a loop that will not stop. Each step of an agent loop is a model call that costs time and money, and a cycle with no cap spins until the machine or the budget gives out. This post caps it with the loop engine's circuit breaker, then closes Part 3 by pasting the board, where every model-surface attack from the red baseline is now green."
date: 2026-01-06
draft: true
tags: ["ai-security", "golang", "agents", "denial-of-service"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The attack that never finishes

The last two posts fenced what the agent may do and who has to approve it. This one is about an abuse that slips past both, because it never tries to finish an action and never oversteps a capability. It just refuses to stop. An agent that moves from one step to the next, plan then act then plan again, can be pushed into a cycle that never reaches an end, by a bug in its routing or by an instruction that keeps it busy. Each trip around that cycle is real work: a model call, a tool run, time and money spent. With nothing to stop it, it runs until the machine falls over or the bill arrives. That is denial of service aimed at the agent itself, OWASP LLM10, and it is the last control in this part. It is deterministic, no model:

```bash
cd posts/06.3-circuit-breakers
go run .
```

## I. A loop with no off switch

Here is the loop the way you might first write it: go from plan to act and back, forever, with no condition that ever ends it. I am not going to actually run that forever, so the demo puts a ceiling on it purely so this page cannot hang your machine. Reaching the ceiling is the point: it means the loop never decided to stop on its own.

```
== before: a loop with no cap ==
  the loop ran 100000 steps and still had not finished (stopped only by the demo's ceiling): true
  in the real agent each step is a paid model call, so this is denial of wallet
```

A hundred thousand steps and still going, stopped only because I refused to let the demo run longer. In the real agent that ceiling is not there, and each of those steps is a call to a model that costs you, so the loop does not just hang, it spends. Nothing in the loop itself was ever going to say enough.

## II. A hard cap on the work

The agent does not run its loop by hand. It runs on `gonductor`, a small dependency-free engine that drives the multi-step plan-approve-act loop as a graph of nodes, and the engine owns the one thing a hand-written loop forgot: a hard limit on how many steps a single run may take. Exceeding it does not warn or slow down, it fails the run with a circuit-breaker error, so a routing cycle that would otherwise spin forever is stopped dead. It is defense in depth by design: the engine-level cap is the hard backstop, and a node can also watch its own step count and route to a graceful halt before the cap is reached. What it defends against is exactly the runaway above: no matter why the loop will not end, it cannot run more than the cap allows.

Run the same endless cycle on the engine, with the cap set low so you can see it bite:

```
== after: the loop engine's circuit breaker ==
  the same cycle, run with a hard cap of 8 steps:
  it stopped after 8 steps with: gonductor: circuit breaker tripped (max steps exceeded)
  circuit breaker tripped: true
```

Eight steps and then a clean stop with a circuit-breaker error. The loop that ran a hundred thousand steps without finishing is held to eight, because the limit lives in the engine and not in the hope that the loop will end itself. The denial-of-wallet attack has a ceiling now, and it is one you set.

## The board, at the end of Part 3

![The architecture so far: the defense layers built through Part 3, solid, with the newest highlighted; the layers still ahead faded.](spine-sofar.svg)

*The architecture so far. Everything solid is built and attacked; P3 is what this part added. The faded layers are the map of what is still ahead.*

This post closes the part, so it is time to look at the scorecard the whole book is built to turn green. Back in [the baseline post](/blog/attacking-the-baseline/) the undefended agent lit up red across the model surface. Here is the board now, after three parts of work:

```
id     slug                               part                     result
00.1   the-plan                           P0 front matter          intro
00.2   setting-up-the-lab                 P0 front matter          intro
00.3   attack-driven-design-shown-once    P0 front matter          method
01.1   one-shape-for-everything           P1 ch1 parser            feature
01.2   dates-the-model-never-decides      P1 ch1 parser            green
01.3   inert-output-and-the-human-accept  P1 ch1 parser            green
01.4   the-watched-folder                 P1 ch1 parser            green
02.1   the-eval-harness                   P1 ch2 prove it          method
02.2   attacking-the-baseline             P1 ch2 prove it          problem
03.1   a-log-you-can-trust                P2 ch3 observability     green
03.2   a-gateway-you-cannot-route-around  P2 ch3 observability     green
03.3   one-trace-id                       P2 ch3 observability     method
04.1   when-the-email-gives-orders        P3 ch4 injection         green
04.2   the-email-is-data-not-orders       P3 ch4 injection         green
04.3   system-prompt-leakage              P3 ch4 injection         green
05.1   scrubbing-secrets-and-pii          P3 ch5 secrets and output green
05.2   the-calendar-field-exfil           P3 ch5 secrets and output green
05.3   the-tracking-pixel                 P3 ch5 secrets and output green
05.4   a-search-tool-hits-a-local-server  P3 ch5 secrets and output green
06.1   least-privilege                    P3 ch6 agency            green
06.2   the-human-gate                     P3 ch6 agency            green
06.3   circuit-breakers                   P3 ch6 agency            green

green 15  red 0  intro 2  feature 1  method 3  problem 1  not yet 39
```

Every lesson that has a control runs green, and `make board` re-runs all of them on every push, so a line that went green cannot quietly go red again without the gate catching it. The injection, the leaks, the exfiltration, the over-agency, the runaway loop, each one landed against the agent at the start of this book and each one now ends on a refusal you can reproduce. The rows still marked `not yet` are the next three parts, and the rows labelled intro, feature, method, or problem are the posts that set up or teach rather than defend.

## Conclusion

That closes Part 3, and with it the part of the agent you can secure without ever letting it run code. The model surface is covered: an email can no longer give the agent orders, fish out its instructions, smuggle data in or out, overstep a tool's privilege, forge an approval, or spin it in place. All of it was proven against the real model, and all of it is held by a gate that runs before every push.

Part 4 is where the stakes change kind. The agent is about to get a tool that executes code, and an email that can reach an executor is not the same class of problem as one that can reach a fetcher. A regular expression over a URL is no longer a wall. The next posts build real ones: an isolated machine with no network card where untrusted-influenced code can run and touch nothing, a two-model design that keeps the part reading hostile text from ever being the part that acts, and the broker that is the single audited way anything gets out. It opens by showing, with nothing between the model and a shell, exactly how bad the unguarded version is.

## Take it to your own project

Cap what a single run can do:

- Set a hard cap on steps, tokens, tool calls, and wall-clock time per run.
- Trip a breaker when a cap is hit, and fail closed.
- Alert on a trip; a runaway loop is a signal, not just noise.
- Test it with an input designed to loop forever, and confirm it stops.
