# SPEC 10.4-memory-poisoning

- part: P5 ch10 RAG and memory
- title: Memory poisoning and lifecycle
- spec: memory-poisoning -> TTL, scope, sanitized recall -> 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
