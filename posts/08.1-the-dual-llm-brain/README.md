# 08.1-the-dual-llm-brain

The model that reads untrusted text produces only data and cannot act; the model
that acts never reads the untrusted text. So a command hidden in an email reaches
only the reader, where it is data, never a plan step. This models the agent's real
control-plane separation (planner vs quarantined coder) as its one structural
invariant.

Run it:

```bash
go run .
```
