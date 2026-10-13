# SPEC 15.2-exactly-once-side-effects

- part: P7 ch15 reliability
- title: Exactly-once side effects
- spec: duplicate-side-effect -> idempotency -> once

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
