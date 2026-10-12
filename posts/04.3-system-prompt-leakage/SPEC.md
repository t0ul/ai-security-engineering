# SPEC 04.3-system-prompt-leakage

- part: P3 ch4 injection
- title: System-prompt leakage
- spec: prompt-leak seed coaxes instructions out -> DetectPromptLeak on output -> LLM07 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
