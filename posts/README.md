# posts/

One folder per post in the AI Security Engineering series. Each folder is a
self-contained lesson you can read and run on its own. `MANIFEST.md` is the book's
order and each lesson's one-line brief.

## Folder layout

```
posts/<id>-<slug>/
  index.md           the post prose (the source of truth for the writing)
  SPEC.md            the brief this lesson was built to (copied from MANIFEST.md)
  go.mod             its own tiny module; dependencies are only the library this post needs
  main.go            a runnable demo: the undefended path, then the defended path
  contract_test.go   the same two paths as tests, so CI agrees with the post
  attack/            the attack input(s) or seeds this post runs
  receipt.txt        the real output of `go run .`, which the prose quotes
  README.md          one line on what to run and what you should see
```

Ids are zero-padded chapter.post (`03.1`, `10.2`) so folders sort in book order.
`_TEMPLATE/` starts with an underscore, so the Go tool ignores it.

## Working on a lesson

```bash
make status                       # what exists, what has code, what has a receipt
make new POST=03.1-a-log-you-can-trust   # scaffold from _TEMPLATE + its manifest row
make check POST=03.1-a-log-you-can-trust # the gate: prose lint + vet + contract tests
make capture POST=03.1-a-log-you-can-trust  # run it, save the real output to receipt.txt
make board                        # re-attack every lesson so far; one row each, green or RED
make pr POST=03.1-a-log-you-can-trust    # branch, check, commit only that folder, open the PR
```

One folder per pull request. `make hooks` installs a pre-push hook that runs
`make check`, so a regression cannot leave your machine; that local gate is the
point of the book. (GitHub Actions runs the same check on pull requests as a
backstop.) The gate tests reality: lessons that involve the model run against the
local model server from post 00.2, and a contract test that cannot reach it fails
rather than skips.

## The before/after contract

Every control post's folder must do two things when you run it:

1. Run the attack against the undefended code. It lands, and the demo prints the
   breach (the forged record, the leaked secret, the event that was never in the
   email).
2. Run the same attack against the fixed code. It is blocked, and the demo prints
   the refusal (the 400, the dropped item, `chain_ok=false` caught).

`contract_test.go` asserts both. The post pastes the real output from
`receipt.txt` as its receipt. The undefended path is genuinely undefended; it omits
the control, rather than faking the result with a stub.

## Rules

- Self-contained: `cd posts/<id>-<slug> && go run .` works with nothing else set up
  (beyond the lab from post 00.2 when a model is involved).
- Imports the real library (for example `github.com/t0ul/gledger`), never a copy, so
  a library change breaks the build and CI catches the drift.
- Minimal: only the code this one lesson needs, not a full working app at every
  step.
- Introduce each library plainly in the prose: what it is, why it is here, and the
  security problem it solves, woven into sentences.
- Prose style is checked by `make lint` (no em dashes, no scaffolding labels, no
  banned phrases, front matter present). Match the voice of the existing posts.

## Publishing

`index.md` is the source of truth. It is published to t0ul.com
`content/blog/<slug>/` when the post is approved.
