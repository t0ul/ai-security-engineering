# 04.1-when-the-email-gives-orders

Indirect prompt injection: an email tells the model to add an event that is not in
it. Undefended, the real local model obeys. Defended, the agent's real controls
(guard.Sanitize, the spotlighting prompt, and reconcile) strip it first and the
attack produces nothing.

Run it (needs the local model from post 00.2):

```bash
go run .
go test -tags live -v ./...   # the live gate; CI runs only the offline guard test
```
