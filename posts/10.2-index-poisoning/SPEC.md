# SPEC 10.2-index-poisoning

- part: P5 ch10 RAG and memory
- title: Index poisoning
- spec: rag-poisoning planted doc steers answers -> XML-encapsulated chunks -> 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
