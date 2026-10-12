# SPEC 01.4-the-watched-folder

- part: P1 ch1 parser
- title: The watched folder
- spec: watcher -> oversized file + half-written read -> size cap + settle delay + per-cycle cap

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
