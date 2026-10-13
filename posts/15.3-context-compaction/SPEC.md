# SPEC 15.3-context-compaction

- part: P7 ch15 reliability
- title: Context compaction and summarization injection
- spec: summarization-injection -> trusted-summary rules -> 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
