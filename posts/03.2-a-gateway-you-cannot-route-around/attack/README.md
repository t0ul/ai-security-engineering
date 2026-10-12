# the attacks

No input files; the attacks are HTTP requests fired at a proxy in-process.

- SSRF: a request that names an internal service as its upstream. A naive proxy
  that trusts the caller's chosen host fetches it. The point is a model call that
  reaches somewhere it was never meant to.
- Flood: a bounded burst of requests (ten, on a throwaway local server, never a
  real one). Enough to show a naive proxy accepting all of them with nothing
  saying stop, and the gateway refusing once past its rate cap.
- Oversized body: one request with a body past the cap.
