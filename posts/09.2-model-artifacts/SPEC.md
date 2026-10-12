# SPEC 09.2-model-artifacts

- part: P4 ch9 protocol and supply chain
- title: Model artifacts
- spec: malicious .pkl vs verified .gguf -> magic + hash check -> pickle 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
