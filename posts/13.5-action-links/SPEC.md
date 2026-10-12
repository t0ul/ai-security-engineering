# SPEC 13.5-action-links

- part: P6 ch13 capstone
- title: Action links
- spec: HITL + netpolicy + VM fetch -> action-link-exfil -> 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
