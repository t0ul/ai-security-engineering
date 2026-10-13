# SPEC 14.3-admin-audit-and-rollback

- part: P7 ch14 governance
- title: Admin audit and one-click rollback
- spec: bad change -> rollback cleanly -> audited

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
