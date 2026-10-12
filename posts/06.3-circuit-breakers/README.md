# 06.3-circuit-breakers

An agent loop that can cycle without reaching a finish is a denial-of-service: each
step is a paid model call, and a cycle with no cap spins forever. A naive loop never
stops itself; the agent's real loop engine enforces a hard step cap that trips a
circuit breaker and stops it. Closes Part 3.

Run it:

```bash
go run .
```
