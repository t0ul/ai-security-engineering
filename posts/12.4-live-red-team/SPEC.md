# SPEC 12.4-live-red-team

- part: P6 ch12 flywheel
- title: A live red-team against the real model
- spec: the live suite is in the gate from day one (local model, seconds to start, never skipped) -> why a suite that stubs the model is not a test suite -> the same attacks against the model itself -> live receipt

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
