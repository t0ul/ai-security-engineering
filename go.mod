module github.com/t0ul/ai-security-engineering

go 1.27.1

require (
	github.com/Code-Hex/go-infinity-channel v1.0.0 // indirect
	golang.org/x/mod v0.22.0 // indirect
)

require (
	github.com/Code-Hex/vz/v3 v3.8.0
	github.com/t0ul/gledger v0.0.0-00010101000000-000000000000
	github.com/t0ul/goflage v0.0.0-00010101000000-000000000000
	github.com/t0ul/gonductor v0.0.0-00010101000000-000000000000
	github.com/t0ul/gorauder v0.0.0-00010101000000-000000000000
	github.com/t0ul/gumpers v0.0.0
	golang.org/x/sys v0.48.0
	golang.org/x/term v0.46.0
)

replace (
	github.com/t0ul/gledger => ../fleet/gledger
	github.com/t0ul/goflage => ../fleet/goflage
	github.com/t0ul/gonductor => ../fleet/gonductor
	github.com/t0ul/gorauder => ../fleet/gorauder
	github.com/t0ul/gouncer => ../fleet/gouncer
	github.com/t0ul/goverlord => ../fleet/goverlord
	github.com/t0ul/gumpers => ../fleet/gumpers
)
