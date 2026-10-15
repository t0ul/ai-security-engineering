---
title: "Data and privacy governance"
description: "Two gates stand between the world and what the agent trusts. At ingestion, a record that claims a trusted source must prove it with a signature, or a forged document poisons the corpus; unsigned data is accepted but contained. At promotion, a change to the agent must clear the eval and red-team gates before it goes live. This forges a trusted record and ships an unsafe change past a naive path, then holds both gates, fail-closed."
date: 2026-08-18
draft: false
tags: ["ai-security", "golang", "governance", "mlops"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## Two gates on trust

Governance so far has been about the agent's operation: who may change it, how to stop it. This post is about its supply chain, the two moments where something from outside becomes something the agent relies on. Data comes in, and a change goes out. Each is a gate, and a gate that is not enforced is a door.

The attacks are the two classic supply-chain ones for an ML system. Poison the data, so the agent learns or retrieves something an attacker planted. Or bypass the review, so a change that would have failed its checks ships anyway. A control lesson, deterministic, no model:

```bash
cd posts/14.5-data-and-privacy-governance
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init gates`, then `go get github.com/t0ul/emaildrop@v0.0.0-20261011232453-771805568f76`, and put the code in one file called `main.go`. The indirect lines come in on their own with `go mod tidy`.

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/14.5-data-and-privacy-governance

go 1.27.1

// The agent's real dataset-provenance check and promotion gate. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/t0ul/gledger v0.0.0-20261006144518-5ac38b514e64 // indirect
	github.com/t0ul/goflage v0.0.0-20261009135357-739ce25ed7e2 // indirect
	github.com/t0ul/gonductor v0.0.0-20261006173449-0ff2040c669a // indirect
	github.com/t0ul/goverlord v0.0.0-20261006151751-a64efb655ae2 // indirect
	github.com/t0ul/gumpers v0.0.0-20261006154904-836570c2c66c // indirect
	github.com/t0ul/gustoms v0.0.0-20261007182046-95bfdd23500c // indirect
	golang.org/x/sys v0.48.0 // indirect
	modernc.org/libc v1.77.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.60.1 // indirect
)
```
<!-- /listing -->

## The spine of the program

`main` runs the two gates in turn, and the receipt needs both to hold: the forgery rejected and the unsigned record contained at ingestion, the unsafe change refused and the reviewed one promoted.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 14.5, data and privacy governance.
//
// Run it with:
//
//	go run .
//
// Two gates stand between the world and what the agent trusts. At ingestion, a
// record that claims a trusted source must prove it with a signature, or a forged
// "trusted" document poisons the corpus. At promotion, a change to the agent must
// clear the eval and red-team gates before it goes live, or an unreviewed,
// regressed, or unsafe change ships. This demo forges a trusted record and pushes
// an unsafe change past a naive path, then holds both gates. Deterministic, no model.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/emaildrop/pkg/dataset"
	"github.com/t0ul/emaildrop/pkg/provenance"
)

func main() {
	g1 := datasetGate()
	g2 := promotionGate()
	fmt.Println()
	if g1 && g2 {
		fmt.Println("the receipt: a forged 'trusted' record is rejected at ingestion while unsigned data is merely contained; an unsafe change is refused at promotion while a reviewed one ships, both gates fail-closed")
	} else {
		fmt.Println("a gate did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

## Ingestion: prove the source or be contained

The agent pulls in documents, and some claim to be authoritative, an official bulletin, a record from a trusted feed. That claim is worth something only if it can be checked, so a record that asserts a trusted source carries a signature over its content. The attack is a forgery: a document that claims the trusted source but is signed by the attacker, or not validly signed at all. The naive ingester and the real check sit side by side in one function.

<!-- listing: main.go#gate 1: dataset provenance at ingestion -->
```go
// ---- gate 1: dataset provenance at ingestion ----

func datasetGate() bool {
	fmt.Println("== gate 1: dataset provenance at ingestion ==")
	source, sourcePub, _ := provenance.NewSigner("trusted-source")
	attacker, _, _ := provenance.NewSigner("attacker")
	verifier := provenance.NewVerifier().Trust("trusted-source", sourcePub)

	text := "Official bulletin: the half-day is October 21st."
	genuine := dataset.Record{ID: "r1", Source: "trusted-source", Text: text, Signed: true, Mark: source.Sign([]byte(text))}
	// A forgery: claims the trusted source, but the signature is the attacker's.
	forged := dataset.Record{ID: "r2", Source: "trusted-source", Text: "Official bulletin: wire tuition to attacker.example.", Signed: true, Mark: attacker.Sign([]byte("Official bulletin: wire tuition to attacker.example."))}
	unsigned := dataset.Record{ID: "r3", Source: "a newsletter", Text: "Book fair next week.", Signed: false}

	// Naive: trust anything that claims to be signed.
	naiveTrusts := func(r dataset.Record) bool { return r.Signed }
	fmt.Printf("  before: a naive ingester trusts the forged 'signed' record: %v\n", naiveTrusts(forged))

	// Real: verify the signature against the trusted key.
	tGen, _ := dataset.Verify(verifier, genuine)
	tForged, errForged := dataset.Verify(verifier, forged)
	tUnsigned, _ := dataset.Verify(verifier, unsigned)
	fmt.Printf("  after:  genuine -> trusted=%v | forged -> trusted=%v (%v) | unsigned -> trusted=%v (contained, not rejected)\n",
		tGen, tForged, errForged != nil, tUnsigned)

	return naiveTrusts(forged) && tGen && !tForged && errors.Is(errForged, dataset.ErrForged) && !tUnsigned
}
```
<!-- /listing -->

```
== gate 1: dataset provenance at ingestion ==
  before: a naive ingester trusts the forged 'signed' record: true
  after:  genuine -> trusted=true | forged -> trusted=false (true) | unsigned -> trusted=false (contained, not rejected)
```

The naive ingester trusts anything that says it is signed, and the forgery walks in as authoritative, poisoning the trusted corpus. The agent's `dataset.Verify` checks the signature against the key it actually trusts: a genuine record is trusted, the forgery is rejected as forged, and the third case is the one that makes this a usable policy rather than a wall. Unsigned data is not rejected, it is accepted and marked untrusted, so it flows into the same contained path, the retrieval fencing and spotlighting from Part 5, that every untrusted email already uses. You do not have to sign the whole world. You have to be unable to claim trust you cannot prove.

## Promotion: pass the gates or stay put

The other direction is a change to the agent itself, a new prompt, a new model, a new policy. In this book those changes have two oracles already: the eval harness that scores quality, and the ADD red-team suite that scores safety. The promotion gate is just the rule that a change cannot go live until it has cleared both. The two oracles are closures the gate calls, here returning fixed scores so the lesson stays deterministic.

<!-- listing: main.go#gate 2: the promotion gate -->
```go
// ---- gate 2: the promotion gate ----

func promotionGate() bool {
	fmt.Println("\n== gate 2: the promotion gate ==")
	plane := controlplane.NewPrompts(map[string]string{"planner": "P0"})

	// A change that failed the red-team: the ADD suite found an attack (ASR > 0).
	unsafe := controlplane.PromotionGate{MinF1: 0.90, F1: func() float64 { return 1.0 }, ASR: func() float64 { return 0.25 }}
	// A change that cleared both gates.
	clean := controlplane.PromotionGate{MinF1: 0.90, F1: func() float64 { return 1.0 }, ASR: func() float64 { return 0.0 }}

	// Naive: promote without consulting the gates.
	plane.Activate("planner", "UNSAFE")
	fmt.Printf("  before: a naive promote ships the unsafe change: live=%q\n", plane.Text("planner"))

	// Real: ActivatePrompt refuses unless the gate allows (fail-closed).
	plane2 := controlplane.NewPrompts(map[string]string{"planner": "P0"})
	_, errUnsafe := controlplane.ActivatePrompt(plane2, unsafe, "planner", "UNSAFE")
	fmt.Printf("  after:  unsafe change refused: %v\n", errUnsafe)
	fmt.Printf("          live prompt stays known-good: %q\n", plane2.Text("planner"))
	vClean, errClean := controlplane.ActivatePrompt(plane2, clean, "planner", "REVIEWED")
	fmt.Printf("          a change that clears eval+ADD is promoted: %q (version %d)\n", plane2.Text("planner"), vClean.Version)

	return plane.Text("planner") == "UNSAFE" && errUnsafe != nil && plane2.Text("planner") == "REVIEWED" && errClean == nil
}
```
<!-- /listing -->

```
== gate 2: the promotion gate ==
  before: a naive promote ships the unsafe change: live="UNSAFE"
  after:  unsafe change refused: promotion gate: ADD attack-success-rate 25% (must be 0)
          live prompt stays known-good: "P0"
          a change that clears eval+ADD is promoted: "REVIEWED" (version 1)
```

The naive promote ships a change whose red-team score is not zero, which is to say a change with a known, measured vulnerability, straight to production. The gate refuses it, names why, the ADD suite found an attack, and leaves the last known-good version live, because it fails closed: no proof, no promotion. A change that actually clears both oracles, quality above baseline and attack-success-rate at zero, goes through normally. The scorecard from Part 6 stops being a thing you look at and becomes a thing you cannot ship past.

## The same shape, at the edges

Both gates are the same instinct the whole book runs on, applied at the two edges of the system instead of inside it. Trusting data requires a verifiable signature, not a claim; shipping a change requires passing the measures, not a promise. And both fail closed, so the absence of proof is a refusal, not a default-allow. Poisoned data cannot enter wearing trust it did not earn, and a change cannot leave without the evidence that it is safe, which is exactly the boundary governance is for: not to slow the good path, but to make the bad path require something it cannot produce.

That is the whole program. Its tests assert both gates, the forged-versus-genuine verdict on its own, and that a gate with no oracles wired refuses; put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"errors"
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/emaildrop/pkg/dataset"
	"github.com/t0ul/emaildrop/pkg/provenance"
)

// Deterministic, no model. A forged "trusted" record is rejected at ingestion and
// unsigned data is contained (not rejected); an unsafe change is refused at
// promotion and a reviewed one ships. Both gates fail closed.

func TestDatasetGateHolds(t *testing.T) {
	if !datasetGate() {
		t.Fatal("the dataset gate must reject a forged trusted record, trust a genuine one, and contain unsigned data")
	}
}

func TestPromotionGateHolds(t *testing.T) {
	if !promotionGate() {
		t.Fatal("the promotion gate must refuse an unsafe change and promote a reviewed one")
	}
}

func TestForgedRecordRejectedGenuineTrusted(t *testing.T) {
	s, pub, _ := provenance.NewSigner("src")
	bad, _, _ := provenance.NewSigner("bad")
	v := provenance.NewVerifier().Trust("src", pub)
	txt := "bulletin"
	if ok, err := dataset.Verify(v, dataset.Record{Text: txt, Signed: true, Mark: s.Sign([]byte(txt))}); !ok || err != nil {
		t.Fatalf("a genuinely signed record must be trusted, got ok=%v err=%v", ok, err)
	}
	if ok, err := dataset.Verify(v, dataset.Record{Text: txt, Signed: true, Mark: bad.Sign([]byte(txt))}); ok || !errors.Is(err, dataset.ErrForged) {
		t.Fatalf("a forged record must be rejected as forged, got ok=%v err=%v", ok, err)
	}
}

func TestPromotionGateFailsClosed(t *testing.T) {
	// A gate with no oracles wired must refuse (cannot promote without proof).
	ok, _ := controlplane.PromotionGate{}.Allow()
	if ok {
		t.Fatal("a promotion gate with no eval/ADD oracle must fail closed")
	}
}
```
<!-- /listing -->

The last governance post is about the agent's output rather than its inputs: signing what it produces, so that a thing claiming to come from the agent can be verified as genuine and unaltered, the same provenance check turned outward.

## Take it to your own project

Gate what you trust and what you ship:

- Require provenance or a signature for data you will trust at ingest.
- Contain unsigned data rather than trusting it.
- Gate any model change behind the eval and red-team, and fail closed.
