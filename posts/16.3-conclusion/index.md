---
title: "Conclusion: what you built, and what's next"
description: "The whole system laid end to end, from the boring core to the governed, isolated, continuously red-teamed agent, the red scorecard now green, the method you keep, and an honest backlog for what comes after the last chapter."
date: 2026-10-13
draft: false
tags: ["ai-security", "golang", "agents"]
series: "AI Security Engineering"
weight: 1700
part: "Conclusion"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## You built an agent that does not get owned

Go back to the first chapter and the want was ordinary: read the school email, put the real dates on the calendar, keep it on my own machine. What stood between that want and a safe version of it was most of AI security, and you have now walked the whole distance. The agent reads text from people it cannot trust, holds private data, and uses tools that act, and it does all three without handing an attacker the keys. That is the finished thing.

## What you built

Laid end to end, here is the system you now have, in the order you built it:

- **A boring, honest core.** A deterministic parser, a single typed event, output that changes nothing on its own, and a human accept step. The model never decides a date it cannot ground in the text.
- **A lens before the defenses.** A tamper-evident audit log, a gateway no call can route around, and one trace id that ties a run together, so nothing after this is invisible.
- **The model surface, closed.** Indirect injection neutralized, the instruction hierarchy enforced, prompt-leak caught, secrets and PII scrubbed at ingest, field-level exfil and tracking pixels blocked, least privilege on every tool, a human gate on anything irreversible, and circuit breakers on runaway loops.
- **Real isolation.** A detonation chamber with no network card, a single audited broker for the rare legitimate call out, no shell, a privileged planner split from a quarantined reader, a pinned tool catalog, verified model artifacts, and the lethal trifecta taken apart one leg at a time.
- **Memory without a way in.** Retrieval that cannot be poisoned or leaked across people, and data the agent refuses to get wrong or make up.
- **A flywheel.** One command that scores the whole thing, the attacks extracted into a reusable library, an honest eval methodology, a live red-team against the real model, and the same method proven on an app that is not this one.
- **Operations and reliability.** A governed control plane, a non-human identity with revocable credentials, admin audit and rollback, a kill switch, data and output governance, and the durable-execution work that keeps all of it from quietly breaking.

And the receipt that matters: the red scorecard from early in the book, where every attack landed, now runs green end to end, every break re-attacked and held.

## The method is the part you keep

The agent was the vehicle. Attack-Driven Design was the point. You did not read that injection defenses work; you watched an injection land, closed it, and watched it fail. You have a loop now, build it, break it, fix it, prove it, that does not care whether the next thing you build is an email agent. The book pointed it at a second, unrelated app to show the transfer, and in the last chapter let a model play attacker against the finished product and hunt for planted flags. Point the same loop at whatever you ship next.

## What's next

A short, honest backlog if you want to keep going:

- **Run the loop on your own app.** Take the plainest thing you are building, point the attacks at it with no defenses, and let it go red. That red board is your table of contents.
- **Make the red-team continuous.** Keep the attacks in the test suite, model included, and run them on every push. A line that went green should not be allowed to go quietly red again.
- **Harden the operation, not just the code.** Governed prompts, scoped credentials, audit and rollback, and a kill switch are what turn a safe demo into something you can actually run for other people.
- **Invest in reliability.** Durable execution, exactly-once side effects, and context compaction are where a working agent fails in boring, expensive ways. They are security too.
- **Give back.** The agent is open at [t0ul/emaildrop](https://github.com/t0ul/emaildrop) and the lessons at [t0ul/ai-security-engineering](https://github.com/t0ul/ai-security-engineering). Open a pull request with an attack I missed or a control done better.

## Thank you

If this book did its job, you are not holding a set of opinions about AI security. You are holding a working agent and a method that proves its defenses on demand. Keep it small, add one control per feature, and make every defense end on a number you can reproduce. Thanks for building it with me, and keep shipping, safely.

## Take it to your own project

The whole method, as a checklist you can keep:

- Build the thing, then attack it with the real tool against the real code.
- Add the real control, and prove the attack stops, measured, every time.
- Keep the loop in the gate so a regression goes red before a push.
- Keep the model, on both sides, something you never take at its word.
- Point this same loop at whatever you build next.
