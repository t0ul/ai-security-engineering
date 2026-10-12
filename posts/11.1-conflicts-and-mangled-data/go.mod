module github.com/t0ul/ai-security-engineering/posts/11.1-conflicts-and-mangled-data

go 1.27.1

// The agent's real deterministic integrity checks: date/weekday/time parsing,
// title normalization, and the conflict detector. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76

require (
	github.com/t0ul/goflage v0.0.0-20261009135357-739ce25ed7e2 // indirect
	github.com/t0ul/gumpers v0.0.0-20261006154904-836570c2c66c // indirect
)
