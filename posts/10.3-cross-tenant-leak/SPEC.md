# SPEC 10.3-cross-tenant-leak

- part: P5 ch10 RAG and memory
- title: Cross-tenant leak
- spec: rag-tenant-leak -> authz-first retrieval, ACL in the query -> 0%

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
