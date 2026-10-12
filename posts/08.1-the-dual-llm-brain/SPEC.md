# SPEC 08.1-the-dual-llm-brain

- part: P4 ch8 dual-LLM
- title: The dual-LLM brain (CaMeL)
- spec: privileged planner decides, quarantined reader touches untrusted text but cannot act -> model output is data

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
