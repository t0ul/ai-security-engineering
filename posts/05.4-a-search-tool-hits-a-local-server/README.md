# 05.4-a-search-tool-hits-a-local-server

The agent's first tool that reaches out, a web search, is server-side request
forgery waiting to happen when the URL comes from a model that read an untrusted
email. A naive fetcher reaches the machine's own internal service; the real egress
policy refuses loopback and the cloud-metadata address while still allowing a
legitimate host. Sets up Part 4.

Run it:

```bash
go run .
```
