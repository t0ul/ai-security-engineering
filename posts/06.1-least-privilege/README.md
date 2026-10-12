# 06.1-least-privilege

Each tool declares one capability: read-only or write. A naive runner writes
whatever any tool returns, so a subverted read-only tool can write a file. The
agent's real pipeline enforces the declared capability and refuses the write, while
a proper write tool still produces its .ics.

Run it:

```bash
go run .
```
