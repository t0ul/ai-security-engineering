# 04.2-the-email-is-data-not-orders

The same injected email as the last lesson, but this time the order is not stripped,
it is demoted. Handed to the model with no separation, the sender's "new task" line
is obeyed. Enclosed in a delimited channel that declares the email untrusted, with
the system message holding sole authority, the same line is ignored. A prompt-level,
model-dependent layer that complements the deterministic sanitizer.

Run it (needs the local model from post 00.2):

```bash
go run .
go test -tags live -v ./...
```
