# SPEC 04.1-when-the-email-gives-orders

- part: P3 ch4 injection
- title: When the email gives orders
- spec: indirect injection found by accident -> 4 seeds land -> guard.Sanitize + gumpers -> 100% to 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
