# 03.1-a-log-you-can-trust

An intruder who can edit the audit log owns the evidence. A plain log accepts the
edit silently; gledger's hash-chained log detects it (chain_ok=false), and redacts
a secret the plain log would write in the clear.

Run it:

```bash
go run .
```
