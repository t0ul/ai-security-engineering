---
title: "The one-command scorecard"
description: "Five parts built a pile of before/after tests. This post turns the pile into one command that scores every control and reds the build when one regresses. But a scorecard is only a gate if each row is wired: the attack has to really land with the control removed, and the control has to really be on the path. The agent's Attack-Driven Design gate enforces exactly that, and catches the two false-green rows this book keeps warning about."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "testing", "attack-driven-design"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## From a pile of tests to one command

Five parts in, the agent is covered by dozens of before/after tests, each one an attack that lands without a control and is blocked with it. That is a lot of green, and green that nobody runs is just a feeling again. Part 6 is about the flywheel: the machinery that keeps all of this true as the agent grows, starting with the simplest piece, a single command that scores every control at once and fails loudly the moment one regresses.

The whole value of that command rests on a claim it is easy to get wrong, and this book has leaned on it since the injection chapter: a test only counts if it can fail. A scorecard full of rows that cannot fail is worse than no scorecard, because it reports safety that was never checked. So before pasting an all-green board and calling it a gate, this post shows what makes a single row trustworthy, and what the agent's real gate does to a row that is not. It is a control lesson, deterministic, no model:

```bash
cd posts/12.1-the-one-command-scorecard
go run .
```

## Three rows that all look green

The agent scores controls with [ADD](https://github.com/t0ul/ADD), a small Go-test library that runs each attack twice, once against the control removed and once against the real control, using the [gorauder](https://github.com/t0ul/gorauder) red-team runner underneath. Here are three rows for the same simple control, a sanitizer that strips a success marker. If you only look at the defended arm, the thing a naive scorecard reports, all three are identical.

```
== before: a scorecard that only checks the defended arm ==
  marker-strip (wired)       defended ASR 0% -> looks GREEN
  marker-strip (stubbed)     defended ASR 0% -> looks GREEN
  marker-strip (not wired)   defended ASR 0% -> looks GREEN
  (all three look identical; two of them are false assurance)
```

Two of those three prove nothing. The stubbed row has an undefended arm that never actually exposes the attack, so of course it reports zero, with or without a control. The not-wired row has a real control that passes its own unit test, but the pipeline it is supposed to protect never calls it, so the real path still leaks. Both show a comforting "defended 0%" and both are lies.

## The gate that cannot be fooled

The ADD gate does not just check the defended arm. For every row it asserts three things: the attack must breach at 100% with the control removed, so the red is real and reachable; the attack must fail at 0% with the control in place; and, when a row supplies a wired integration target, the attack must fail against the real end-to-end path too, which catches a control that exists in the codebase but is not actually called. Run the same three rows through it.

```
== after: the ADD gate's invariants ==
  marker-strip (wired)       PASS: undefended 100% -> defended 0% -> wired 0%
  marker-strip (stubbed)     FLAGGED: attack NOT PROVEN, undefended ASR 0% (want 100%); the test proves nothing
  marker-strip (not wired)   FLAGGED: control NOT WIRED, integration ASR 100% (control exists but the real path isn't protected)
```

Only the wired row passes. The stubbed row is called out for proving nothing, because its undefended arm did not breach. The not-wired row is called out because the control, though it works in isolation, is not on the path that actually runs. This is the discipline from the very first attacks made mechanical: a green row now certifies that the attack was real and that the thing stopping it is wired into the product, not just present in it.

## The board, run before every push

With that guarantee behind each row, the all-green board finally means something, and it is one command. In the book that command is `make board`, which runs every lesson's test; in the product it is `make check`, the deterministic scorecard plus the live model suite plus the unit tests. The same command runs on a git pre-push hook, so a control cannot regress without the push that broke it going red first.

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
12.1  the-one-command-scorecard           green

green 27  red 0  prose 12
```

Twenty-seven wired controls, every one re-attacked and holding, no red. That is the gate. The rest of this chapter builds out the flywheel around it: pulling the attack loop out into the reusable library you just saw, scoring honestly when the judge is itself a model, keeping a live red-team in the gate rather than saving it for a quarterly pentest, and then the real test of whether any of this generalizes, pointing the same method at an app that is not this one.
