# SPEC 13.1-full-chain-attack-contained

- part: P6 ch13 capstone
- title: A full-chain attack, contained
- spec: one poisoned email tries everything -> replay by trace id -> every layer holds

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
