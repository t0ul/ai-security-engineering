---
title: "The app-layer red team: GORED"
description: "Every test in this book was written by the person who wrote the defenses, which means every test shares the author's blind spots. The final move hands the finished product to an adversary that does not: a model-driven red team that plants its own unforgeable flags and drives a small local model to capture them, confirms success with an oracle a self-report cannot fake, and comes up empty once the bug is fixed. The model is the weapon, and this is live by nature, its own exercise, not a gate."
date: 2026-10-13
draft: true
tags: ["ai-security", "golang", "red-team", "offensive"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The author's blind spots

This book has been one person attacking their own work, which is the honest way to learn and the dishonest way to finish. Every attack here was written by the same hand that wrote the defense, so every attack is shaped by the same assumptions, and the vulnerabilities an author cannot imagine are exactly the ones their tests will never contain. A finished system deserves one thing its builder cannot provide: an adversary who does not already know how it is supposed to work.

So the last move turns the book's own thesis on the book. The whole way through, the model has been the thing under attack. Here the model is the attacker. This is GORED, an app-layer red team that uses a small local model as the weapon and a real web app as the target, and because a model-driven attack is live and stochastic by nature, this is its own exercise, run on purpose, never a gate in the pipeline. There is no before-and-after test module here; there is a tool you point at a running app, and these are its real receipts.

## An oracle you cannot talk out of the truth

The first thing a red team against a language model has to solve is that the attacker lies. A model asked to capture a flag will happily announce that it did, whether or not it is true, so success cannot be the model's word for it. GORED's answer is an oracle that holds a fresh random secret per planted flag, generated server-side at the start of each run, and records a capture only when that exact secret is disclosed through the genuine exploit path or submitted back intact. A model that claims victory without actually exercising the bug has disclosed nothing, and the oracle is unmoved. Success is a fact the target reports from the real exploit site, not a sentence the attacker wrote.

The target for this run is a small benign app with a planted object-level access bug, an IDOR: a notes API that checks you have a session but never checks the note is yours, so a low-privilege session can read an admin note that holds the flag. It is the most common serious web vulnerability there is, and it is a good test of whether a cheap model can find and walk through a real one.

## The model captures the flag

Pointed at the vulnerable build, a half-billion-parameter local model, the kind that boots in a couple of seconds, drives the attack and captures the flag.

```
🧾 [oracle] capture_detected
outcome=captured via=idor:GET /api/notes/1000 steps=1 tool_calls=1 wall_ms=255 tokens_in=467 tokens_out=35 findings=1
```

A quarter of a second, one tool call, thirty-five tokens of output. The model recognized that the notes were addressable by id, asked for one it was not supposed to have, and the admin note's flag disclosed itself to a session that did not own it. The oracle recorded the capture at the exploit site, `idor:GET /api/notes/1000`, not from anything the model claimed. A real bug, found and exercised by a model small enough to run on a laptop, with the proof coming from the target rather than the attacker.

That is the uncomfortable half of the lesson. The things a model is good enough to do unsupervised now include finding and exploiting an ordinary access-control bug, quickly and cheaply. The same capability this book spent fifteen chapters containing on the defense is available just as readily to the offense.

## Fix the bug, and it comes up empty

The point of a red team is not the capture, it is what the capture tells you to fix, and whether the fix holds. The hardened build of the same app adds the one control the bug was missing, a per-record ownership check on the notes API, so the low-privilege session is refused the admin note. Run the same hunt against it.

```
TestHuntObjectLevelIDOR            PASS   (vulnerable: flag captured)
TestHuntObjectLevelIDORHardenedClosed   PASS   (hardened: no IDOR reported, flag never discloses)
```

The hardened build gives the red team nothing: the exploit path is closed, the admin note is never served to the wrong session, and the oracle records no capture because there is nothing to disclose. This is the same ASR-one-hundred-to-zero shape as every control in the book, drawn this time by an autonomous attacker against the running app rather than by a unit test against a function. The fix is not proven by my test agreeing with my code; it is proven by an adversary that wanted the flag and could not get it.

## The model on both sides

GORED is a separate project from the agent, deliberately, and it is bigger than one post. What belongs here, at the end of this book, is the symmetry it completes. The series opened with a model reading an untrusted email and a long argument that the model's output must never be trusted as an instruction. It closes with a model reading an untrusted app and the same model's output being trusted least of all, its claims of success ignored in favor of a secret only the target can reveal. Offense and defense turn out to be the same discipline seen from opposite ends: in both, the model is powerful, useful, and fundamentally not to be taken at its word, and the security is in the structure around it that does not have to.

That is the whole method, finished. Build the thing. Attack it with the real tool against the real code, and watch the attack land. Add the real control, and watch it stop, measured, every time. Keep the loop running so a regression turns a line red before a push, and keep the attacker honest with an oracle it cannot forge. Then hand the finished product to something that does not share your blind spots, and let it tell you what you missed. An agent built this way is not an agent that cannot be attacked. It is an agent whose every defense you can point to, reproduce, and prove still holds, which is the most honest thing security engineering has to offer, and the only kind worth shipping.
