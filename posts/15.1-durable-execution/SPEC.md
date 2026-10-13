# SPEC 15.1-durable-execution

- part: P7 ch15 reliability
- title: Durable execution and resume-verify
- spec: resume-tampered-state -> hash-chained steps -> refuse forged resume

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
