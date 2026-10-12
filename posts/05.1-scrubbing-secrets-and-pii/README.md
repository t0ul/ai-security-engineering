# 05.1-scrubbing-secrets-and-pii

The agent indexes every email into a searchable corpus on disk. Index an email full
of secrets as-is and they are all there in the clear; scrub at ingest with goflage
and the recognized ones are gone. Honest about one token format the recognizer
does not know.

Run it:

```bash
go run .
```
