# SPEC 12.1-the-one-command-scorecard

- part: P6 ch12 flywheel
- title: The one-command scorecard, all green
- spec: run the product's full gate: deterministic scorecard + live suite + tests -> every row 100% to 0% -> paste make board, all green -> reproducible in one command, and it runs before every push

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
