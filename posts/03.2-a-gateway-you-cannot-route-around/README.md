# 03.2-a-gateway-you-cannot-route-around

A naive model proxy lets the caller choose the upstream (SSRF) and has no caps
(flood). gouncer allowlists logical model names, caps the body, and rate-limits
per client, so an unknown model is a 400, an oversized body a 413, and a burst a
429. The flood is a bounded 10-request burst on a throwaway server.

Run it:

```bash
go run .
```
