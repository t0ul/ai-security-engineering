# SPEC 03.2-a-gateway-you-cannot-route-around

- part: P2 ch3 observability
- title: A gateway you cannot route around
- spec: gouncer -> SSRF to any host + flood -> model allowlist + body, rate, concurrency caps -> 400 and 429

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
