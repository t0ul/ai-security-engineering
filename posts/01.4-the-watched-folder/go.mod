module github.com/t0ul/ai-security-engineering/posts/01.4-the-watched-folder

go 1.27.1

// The product's real ingestion limits (size cap, settle delay, per-cycle cap),
// imported so the defended path tracks the daemon and a change to either breaks
// this build. No local overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76

require (
	github.com/t0ul/gledger v0.0.0-20261006144518-5ac38b514e64 // indirect
	github.com/t0ul/goflage v0.0.0-20261009135357-739ce25ed7e2 // indirect
	github.com/t0ul/gumpers v0.0.0-20261006154904-836570c2c66c // indirect
)
