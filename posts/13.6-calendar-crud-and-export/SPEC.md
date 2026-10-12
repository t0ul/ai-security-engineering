# SPEC 13.6-calendar-crud-and-export

- part: P6 ch13 capstone
- title: Calendar CRUD and export
- spec: dup-free export -> duplicate-side-effect -> exactly once

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
