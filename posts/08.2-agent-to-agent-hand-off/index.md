---
title: "Agent-to-agent hand-off"
description: "The dual-LLM split only holds if the hand-off between the planner and the executor is authenticated. This post shows a naive executor that runs any plan claiming to come from the planner, forges that hand-off three ways to drive it, then wires the agent's real inter-agent authentication so the executor runs only a plan that verifies as the planner's own, unaltered, and never twice."
date: 2026-02-17
draft: false
tags: ["ai-security", "golang", "agents", "camel", "a2a"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The seam between the two brains

The last post split the brain in two: a planner that decides and a reader that reads untrusted text but cannot act. I ended it by admitting the split rests on something I had not built yet. The two halves have to talk. The planner decides on a plan and the executor carries it out, and between them is a message that says "here is the plan, run it." If the executor will act on any message that claims to be the plan, then the line we drew between reading and acting has a door in it, and anyone on the bus can knock.

That is the shape of this post. I am not attacking a model here; the hand-off is plumbing, and it is deterministic, so this is a control lesson with no model in the loop:

```bash
cd posts/08.2-agent-to-agent-hand-off
go run .
```

## The hand-off, written the easy way

The executor's job is small: take the planner's plan, find the one command the planner chose, run it. Here is the version I would write first.

```go
func executeNaive(h handoff) {
	if cmd := commandIn(h.Plan); cmd != "" {
		runCommand(cmd)
	}
}
```

There is no check that the plan came from the planner. The message carries a signature field, but nothing reads it. In the real agent the command would be detonated in the sandbox; here it is recorded to a slice so you can see exactly what the executor agreed to run, and the attacker's command is a benign marker standing in for the destructive one a real spoof would carry.

## Three ways to forge a plan

The attack is called a2a-spoof, and it does not try to trick the planner at all. It skips the planner and hands the executor a plan directly, forging the message three ways, each one a different lie about where the plan came from. The first claims to be the planner but is signed with a guessed key. The second is signed honestly, but by a rogue agent the executor was never told to trust. The third is the interesting one: it takes the planner's genuine signature over a harmless plan, which the attacker has seen go by on the bus, and pairs that real signature with a different plan for the executor to run.

```
== before: the executor trusts any message claiming to be the planner ==
  forged-sig     executor ran "exfiltrate-inbox-to-attacker": true
  unknown-agent  executor ran "exfiltrate-inbox-to-attacker": true
  plan-swapped   executor ran "exfiltrate-inbox-to-attacker": true
  a2a-spoof ASR: 100% (3/3)
```

All three land. The executor ran the attacker's command every time, because the only question it asked was whether the message had a plan in it, and all three did. The signature on the message was decoration.

## Give the hand-off an identity

The fix is to make the hand-off something only the planner can produce and the executor can check. The agent does this with a small library I wrote called [a2a](https://github.com/t0ul/emaildrop/tree/main/pkg/agent/a2a), for agent-to-agent. It gives each agent an identity and a secret key, signs every message with an HMAC over the sender, a fresh nonce, and the body, and has the receiver verify the signature against the key it holds for that sender before trusting a word of it. An HMAC is a tag that you can only compute if you hold the key, so a signature that checks out is proof the holder of the planner's key produced this exact body, and the nonce, recorded once and refused ever after, means a message that was genuine once cannot be replayed to fire its action a second time.

The planner signs its plan. The executor's gate is two lines, and they are worth reading as a pair, because this is exactly the check the agent's orchestrator makes before it detonates anything:

```go
func executeAuthed(v *a2a.Verifier, h handoff) error {
	if err := v.Verify(h.Sig); err != nil {
		return err
	}
	if h.Sig.Body != h.Plan {
		return errors.New("plan does not match the signed hand-off")
	}
	...
}
```

The first line is authenticity: the signature must verify as the trusted planner's, which also rejects the replay. The second line is the one that catches the clever attack. Verifying a signature only tells you the body that was signed is genuine. It says nothing about the plan the executor is about to run. So the executor also insists that the plan in its hand is the plan that was signed, which closes the gap the third forgery tried to walk through.

## The same three forgeries, now refused

Nothing about the attack changes. The same three messages go to the executor, and this time it authenticates the planner first.

```
== after: the executor authenticates the planner's hand-off ==
  forged-sig     refused: a2a: signature does not verify (forged or tampered)
  unknown-agent  refused: a2a: message from an unknown agent: "rogue"
  plan-swapped   refused: plan does not match the signed hand-off
  a2a-spoof ASR: 0% (0/3)
```

Zero. Each forgery failed at a different line. The guessed key could not produce a signature that verifies. The rogue agent is not someone the executor trusts, so its honest signature buys it nothing. And the stolen-signature-with-a-swapped-plan was caught not by the signature, which was real, but by the executor refusing to run a plan that was not the one signed.

The gate is not just a wall for forgeries, though. A genuine plan still has to get through, and a genuine plan delivered twice must not fire twice:

```
== the genuine planner still gets through ==
  planner's own plan: err=<nil>, executor ran "uname -a": true

== a captured genuine hand-off cannot be replayed ==
  first delivery: err=<nil> | replayed delivery: err=a2a: nonce already seen (replay)
```

The planner's own signed plan runs. The same message delivered a second time is refused, because its nonce was already spent, so an attacker who records a legitimate hand-off cannot replay it later to re-fire the action it carried.

## Authenticate the control flow, not the content

It is worth being clear about what this defends and what it does not. a2a does not read the plan or judge whether it is safe. It answers one question: did this exact plan come from the agent I trust to make plans, and have I seen it before. The content controls are the rest of the book, from spotlighting the untrusted email to confining the command the planner is even allowed to choose. What this post adds is that the decision itself, the hand-off from the brain that decides to the hand that acts, has an identity, so nothing can impersonate the planner and smuggle an action across the boundary the last post built.

That is the seam closed. The planner and the executor now speak on an authenticated channel, and a forged decision gets nowhere. The next target is the tools the executor reaches for once it has a genuine plan. A plan can be perfectly authentic and still call a tool whose definition changed out from under you, so the next post pins the tool catalog and watches what happens when someone swaps it.

## Take it to your own project

Authenticate every hand-off between components:

- Sign the message that crosses from the privileged side to the executor.
- Reject anything that cannot prove it came from the privileged side.
- Do not trust a message because it arrived on the expected channel.
- Test a spoofed hand-off and confirm it is refused.
