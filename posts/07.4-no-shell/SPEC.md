# SPEC 07.4-no-shell

- part: P4 ch7 isolation
- title: No shell
- spec: argv arrays + argcheck allowlist -> allowlist-bypass + arg-injection -> 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
