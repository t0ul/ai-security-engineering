---
title: "Make it idiomatic: the repo refactor"
description: "A secure system you cannot read is a secure system you cannot keep secure. This post is the refactor that turned a pile of controls into Go you would be glad to hand to someone else: small packages with single jobs, security primitives extracted into reusable libraries, and the product split from the handbook. There is no attack here. The point is that every lesson in this book could import a real control because the refactor made the controls real packages."
date: 2026-09-29
draft: true
tags: ["ai-security", "golang", "architecture", "refactoring"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## Readable is a security property

Almost everything in this book has been a control. This post is the one that is not, and it is here on purpose, because the thing that keeps a system secure after the author moves on is not any single control, it is whether the next person can read the code well enough to not break one. A clever defense buried in a thousand-line file nobody wants to touch is a defense on borrowed time. So the last chapter before the finish is about craft: turning the accumulated pile into Go you would be glad to hand over. There is no attack to run, so this one is prose.

## One job per package

The first move was the boring, decisive one: small packages, each with a single job, named for what it does. The security-relevant code in particular lives in packages you can read in a sitting, and a reader can hold the whole thing in their head.

```
pkg/
  agent/
    guard/        injection neutralization and leak detection
    dateparse/    deterministic date parsing + the weekday check
    ics/          inert calendar output
    pipeline/     the capability-enforcing ingestion flow
  controlplane/   the orchestrator, prompts plane, safety gate, authority
  provenance/     content credentials (sign / verify)
  hitl/           the human-approval gate
  netpolicy/      egress allowlist and SSRF defenses
  durable/        the exactly-once, hash-chained step log
  memory/         scoped, expiring, sanitized recall
```

The payoff is that the boundaries in the code are the boundaries in the threat model. The injection defense is one package; the egress defense is another; the human gate is a third. When a reviewer asks "what stops exfiltration", the answer is a directory, not a grep. And because each package does one thing, each is testable in isolation, which is the quiet prerequisite for everything this book did: a control you cannot import and exercise on its own is a control you cannot honestly prove.

## Primitives become libraries

The second move went further than tidying. The pieces that are not specific to an email agent at all, the audit ledger, the PII scrubber, the red-team runner, the capability tokens, the Attack-Driven Design harness, were pulled out into their own modules entirely, with their own names and their own tests. `gledger` for the hash-chained audit, `goflage` for PII, `gorauder` for the red team, the `ADD` library for the gate, and the rest.

This is the one that matters most for the book, and it is worth being plain about why. Every control lesson here imported the real thing and attacked it. That was only possible because the real thing is a package with a public API, not a tangle inside the webapp. The refactor is what made "wired, not stubbed" achievable instead of aspirational: the test in each post could require the actual `guard` or `netpolicy` or `provenance` and run the actual attack against it, because those were, by then, real importable code with no reason to be mocked. The architecture and the testing discipline are the same decision seen from two sides.

## The product and the handbook, apart

The last split was between the product and the writing about it. The agent lives in [t0ul/emaildrop](https://github.com/t0ul/emaildrop); this handbook is its own repository of self-contained lessons, each a tiny module that imports the product and the libraries at a pinned version and runs one before-and-after. Keeping them apart means the book cannot drift from the product: a lesson that still passes is a lesson still wired to code that still behaves, because it builds against the real package every time its test runs.

None of this is novel Go. It is just Go done with the assumption that someone will have to maintain the security of this system without the person who wrote it, which turns out to demand exactly the things good Go wants anyway: small surfaces, clear names, real boundaries, no hidden coupling. Idiomatic and auditable are, more often than not, the same shape.

## The last thing to try

The agent is built, attacked, hardened, governed, made durable, and now readable. One question is left, and it is the one you can only ask of a finished system: what did the author miss. Every test in this book was written by the same person who wrote the defenses, which means every test encodes the same blind spots. The final post hands the finished product to an adversary that does not share them, a model-driven red team that plants its own flags and tries to capture them, and sees what a year of careful building looks like to something actively trying to break it.

## Take it to your own project

Refactor without losing the proof:

- Change the shape for readability, not the behavior.
- Keep every attack green through the refactor.
- Keep the dependency surface small; idiomatic code is auditable code.
