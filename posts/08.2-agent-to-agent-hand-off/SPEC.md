# SPEC 08.2-agent-to-agent-hand-off

- part: P4 ch8 dual-LLM
- title: Agent-to-agent hand-off
- spec: authenticate planner to executor -> a2a-spoof -> 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
