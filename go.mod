module github.com/t0ul/ai-security-engineering

go 1.27.1

require (
	github.com/Code-Hex/go-infinity-channel v1.0.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/mod v0.41.0 // indirect
	modernc.org/libc v1.77.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

require (
	github.com/Code-Hex/vz/v3 v3.8.0
	github.com/t0ul/ADD v0.0.0-00010101000000-000000000000
	github.com/t0ul/GoRag v0.0.0-00010101000000-000000000000
	github.com/t0ul/gledger v0.0.0-00010101000000-000000000000
	github.com/t0ul/goflage v0.0.0-00010101000000-000000000000
	github.com/t0ul/gonductor v0.0.0-00010101000000-000000000000
	github.com/t0ul/gorauder v0.0.0-00010101000000-000000000000
	github.com/t0ul/gouncer v0.0.0-00010101000000-000000000000
	github.com/t0ul/goverlord v0.0.0-00010101000000-000000000000
	github.com/t0ul/gumpers v0.0.0
	github.com/t0ul/gustoms v0.0.0-00010101000000-000000000000
	golang.org/x/sys v0.48.0
	golang.org/x/term v0.46.0
	modernc.org/sqlite v1.60.1
)

replace (
	github.com/t0ul/ADD => ../fleet/ADD
	github.com/t0ul/gledger => ../fleet/gledger
	github.com/t0ul/goflage => ../fleet/goflage
	github.com/t0ul/gonductor => ../fleet/gonductor
	github.com/t0ul/gorauder => ../fleet/gorauder
	github.com/t0ul/gouncer => ../fleet/gouncer
	github.com/t0ul/goverlord => ../fleet/goverlord
	github.com/t0ul/gumpers => ../fleet/gumpers
	github.com/t0ul/gustoms => ../fleet/gustoms
)

replace github.com/t0ul/GoRag => ../fleet/GoRag
