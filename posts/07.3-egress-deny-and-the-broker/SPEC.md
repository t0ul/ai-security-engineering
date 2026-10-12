# SPEC 07.3-egress-deny-and-the-broker

- part: P4 ch7 isolation
- title: Egress-deny and the one way out
- spec: egress denied by default -> allowlist broker is the one audited exit

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
