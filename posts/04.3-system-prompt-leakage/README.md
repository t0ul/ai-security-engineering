# 04.3-system-prompt-leakage

The attack asks the model to recite its own instructions. Undefended, it does, and
hands the system prompt to the user. Defended, a detector notices the response
echoing the prompt and withholds it. The real detector is guard.DetectPromptLeak.

Run it (needs the local model from post 00.2):

```bash
go run .
go test -tags live -v ./...
```
