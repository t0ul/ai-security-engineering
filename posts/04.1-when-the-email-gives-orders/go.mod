module github.com/t0ul/ai-security-engineering/posts/04.1-when-the-email-gives-orders

go 1.27.1

// The agent's real injection controls (guard.Sanitize + the spotlighting prompt +
// ensemble.Reconcile, via the real extractor) and the live model path. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76

require github.com/t0ul/gumpers v0.0.0-20261006154904-836570c2c66c // indirect
