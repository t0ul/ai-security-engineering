# SPEC 03.1-a-log-you-can-trust

- part: P2 ch3 observability
- title: A log you can trust
- spec: gledger -> forge a plain log + the 0-events blindness -> hash chain + redact + defang -> chain_ok=false, trace explains

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
