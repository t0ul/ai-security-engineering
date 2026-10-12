module github.com/t0ul/ai-security-engineering/posts/06.1-least-privilege

go 1.27.1

require (
	// The agent's real tool contract and the pipeline that enforces each tool's
	// declared capability. No overrides.
	github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
	github.com/t0ul/gledger v0.0.0-20261006144518-5ac38b514e64
)

require (
	github.com/t0ul/goflage v0.0.0-20261009135357-739ce25ed7e2 // indirect
	github.com/t0ul/gumpers v0.0.0-20261006154904-836570c2c66c // indirect
)
