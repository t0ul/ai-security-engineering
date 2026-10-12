# 07.3-egress-deny-and-the-broker

The detonation chamber has no network card, so code inside asks a broker on the
host when it needs the network. A naive broker dials whatever it is told and reaches
an internal service; the agent's real broker gates every target through the egress
policy, refusing the internal one and tunneling only an allowlisted host.

Run it:

```bash
go run .
```
