# The AI Security Engineering Handbook

Build a small AI agent, attack it, and harden it, one lesson at a time.

This repo is the companion to the AI Security Engineering series on t0ul.com. It
teaches you to build an agent that reads email and writes calendar events, running
entirely on your own machine, and to make it hard to attack using Attack-Driven
Design: build a feature, attack it with a real tool, watch it fail, add the fix,
and prove the same attack now fails.

## How this repo works

Everything lives under `posts/`. Each folder is one lesson, matching one post in
the series, and is self-contained:

- Read the post (`posts/<id>-<slug>/index.md`).
- Run the folder. It runs an attack against the undefended code and the attack
  lands, then against the fixed code and the attack is blocked.
- That before/after is a real number you can reproduce on your own machine.

See `posts/README.md` for the folder contract, and start with
`posts/0.0-introduction/`.

## The product

The finished tool this book builds toward lives in its own repo,
[t0ul/emaildrop](https://github.com/t0ul/emaildrop). This repo is the teaching
companion: small, self-contained lessons rather than the integrated app. To run
the full agent, clone emaildrop; to follow the lessons, work through `posts/`
here. The anonymized sample emails the lessons use stay in `testdata/`.

## Ground rules

- Local-first. Everything runs on one Apple Silicon Mac, and nothing leaves it.
- Pure Go, with tiny dependency trees.
- Each security control is a small, readable Go library, introduced plainly where
  it is first needed.
