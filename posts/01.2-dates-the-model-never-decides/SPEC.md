# SPEC 01.2-dates-the-model-never-decides

- part: P1 ch1 parser
- title: Dates the model never decides
- spec: dateparse -> stated weekday mismatch + Feb 30 -> naive parse calendars a wrong date -> weekday integrity + reject impossible -> flagged not calendared

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
