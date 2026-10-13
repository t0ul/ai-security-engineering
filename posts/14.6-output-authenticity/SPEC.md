# SPEC 14.6-output-authenticity

- part: P7 ch14 governance
- title: Output authenticity
- spec: sign what the agent produces (C2PA) -> output-forgery -> 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
