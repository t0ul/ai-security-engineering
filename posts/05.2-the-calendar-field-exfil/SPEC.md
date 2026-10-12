# SPEC 05.2-the-calendar-field-exfil

- part: P3 ch5 secrets and output
- title: The calendar-field exfil
- spec: url-exfil via URL and NOTES fields -> field sanitizer -> LLM05 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
