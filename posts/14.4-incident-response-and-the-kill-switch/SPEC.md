# SPEC 14.4-incident-response-and-the-kill-switch

- part: P7 ch14 governance
- title: Incident response and the kill switch
- spec: layered stop -> unauthz-killswitch + deny storm -> contained

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
