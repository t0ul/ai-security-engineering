# SPEC 05.1-scrubbing-secrets-and-pii

- part: P3 ch5 secrets and output
- title: Scrubbing secrets and PII at ingest
- spec: goflage -> pii-secret-leak: tokens and PII reach disk -> scrub at ingest -> none in corpus

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
