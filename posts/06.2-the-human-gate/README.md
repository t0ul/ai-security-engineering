# 06.2-the-human-gate

An irreversible action needs a human's yes, but a one-click yes can be forged or
clickjacked. A naive accept acts on any approval; the agent's real gate acts only on
an approval that echoes a nonce shown with the evidence, so a blind or forged click
cannot pass while a genuine human yes still can.

Run it:

```bash
go run .
```
