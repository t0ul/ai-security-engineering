# SPEC 05.4-a-search-tool-hits-a-local-server

- part: P3 ch5 secrets and output
- title: A web-search tool that hits a local server
- spec: SSRF teaser -> tool reaches 127.0.0.1 -> host policy -> sets up Part 4

Kind: CONTROL. Deliver a runnable before/after.
Done when: go test passes (attack lands undefended, blocked defended),
receipt.txt is captured from a real run, and index.md passes make lint.
