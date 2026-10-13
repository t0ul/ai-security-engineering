---
title: "Incident response and the kill switch"
description: "The kill switch is the control you reach for in an incident, so it has to be two things an ad-hoc flag is not. Governed, because an attacker who can flip the switch can flip it off mid-incident or on as a denial of service. And layered, because a single full-stop is its own outage, a deny storm against every legitimate user. The agent's safety gate requires a break-glass capability to toggle, and stops side effects without stopping reads."
date: 2026-10-13
draft: true
tags: ["ai-security", "golang", "governance", "incident-response"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The control you reach for under pressure

A kill switch has shown up in this book twice already, wired to the capability authority so that engaging it revokes every grant at once. This post is about the switch as an incident-response tool, in the moment you actually use it, because that moment is where a naive kill switch fails in two different ways. Both come from treating it as a simple flag, and both are the kind of thing you only discover at the worst possible time.

The first is that the switch is a control like any other, which means it is a target. The second is that a stop is not free. A control lesson, deterministic, no model:

```bash
cd posts/14.4-incident-response-and-the-kill-switch
go run .
```

## A switch anyone can flip, and only one way to stop

Start with the ad-hoc version: a flag anyone can set, with two positions, running and halted. During an incident, the responder halts the agent. But if the flag is ungoverned, so can the attacker, in the other direction.

```
== before: an ungoverned, all-or-nothing kill switch ==
  attacker flipped the switch back off mid-incident: true
  the only stop is a full halt; a safe read is refused too: true (deny storm)
```

Two failures in two lines. The attacker flipped the switch back off while the incident was still live, because nothing checked that the person toggling it was allowed to. And the responder's only tool was a full halt, which refused everything, including a harmless read of the calendar, so containing one misbehaving tool took the whole agent offline. The incident response became its own denial of service, a deny storm against every legitimate user, which is a strong reason not to pull the switch and exactly the hesitation you cannot afford in an incident.

## Governed to toggle, layered to apply

The agent's safety gate fixes both. Toggling it is a privileged action on a safety resource, checked with the break-glass path of the capability system. Break-glass is deliberately the emergency verifier: it ignores revocation and the halt state, so an operator can always reach the switch even when everything else is frozen, but it still demands the right scope, so an app-scoped role cannot touch it.

```
== after: governed toggle, layered levels ==
  app role tries to toggle: refused: controlplane: grant does not cover requested scope
```

And the switch is not two positions but four, which is what lets a response be proportionate. The level below a full halt is block-tools: sessions keep running and reads keep answering, but no tool, no sandbox, no side effect executes. For an incident that is a misbehaving tool, that is the surgical cut.

```
  responder engages block-tools (authorized): applied=true
    a side-effecting tool: blocked
    a safe read:           allowed   (contained, no storm)
```

The thing causing harm, side effects, is stopped instantly, and the household can still ask what is on the calendar, because reads were never the problem. The responder contained the incident without causing an outage, which means they can reach for the control early instead of weighing it against taking the whole agent down. And when a full stop genuinely is warranted, it is one level up, available to the same authorized operator.

```
  responder escalates to full halt when warranted: everything stopped=true
```

## A stop is a decision, not a flag

The lesson under both failures is that stopping an agent is a governed action with a blast radius, not a boolean. Who may stop it is an authorization question, answered by a capability scoped to the switch, so the control cannot be turned against you. How much to stop is a proportionality question, answered by levels, so the response fits the incident instead of flattening everything. A kill switch built as a flag gets both wrong; built as a governed, layered control, it is something you can actually use the moment you need it, which is the only time it matters.

The last two governance posts move from the agent's operation to its data and its output: the gates that keep a poisoned dataset or an unreviewed model from being promoted, and the signatures that let anyone verify the agent produced a thing, unaltered.
