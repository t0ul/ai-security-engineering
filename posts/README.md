# posts/

One folder per post in the AI Security Engineering series. Each folder is a
self-contained lesson you can read and run on its own.

## Folder layout

```
posts/<id>-<slug>/
  index.md      the post prose (the source of truth for the writing)
  go.mod        its own tiny module; dependencies are only the library this post needs
  main.go       a runnable demo: the undefended path, then the defended path
  attack/       the attack input(s) or seeds this post runs
  README.md     one line on what to run and what you should see
```

Copy `_TEMPLATE/` to start a new one. (`_TEMPLATE/` begins with an underscore, so
the Go tool ignores it.)

## The before/after contract

Every control post's folder must do two things when you run it:

1. Run the attack against the undefended code. It lands, and the demo prints the
   breach (the forged record, the leaked secret, the event that was never in the
   email).
2. Run the same attack against the fixed code. It is blocked, and the demo prints
   the refusal (the 400, the dropped item, `chain_ok=false` caught).

The post pastes that real output as its receipt. The undefended path is genuinely
undefended; it omits the control, rather than faking the result with a stub.

## Rules

- Self-contained: `cd posts/<id>-<slug> && go run .` works with nothing else set up
  (beyond the lab from post 0.2 when a model is involved).
- Imports the real library (for example `github.com/t0ul/gledger`), never a copy, so
  a library change breaks the build and CI catches the drift.
- Minimal: only the code this one lesson needs, not a full working app at every
  step.
- Introduce each library plainly in the prose: what it is, why it is here, and the
  security problem it solves, woven into sentences.

## Publishing

`index.md` is the source of truth. It is published to t0ul.com
`content/blog/<slug>/` when the post is approved. Front-matter and voice rules live
in the project's `docs/VOICE-GUIDE.md`.
