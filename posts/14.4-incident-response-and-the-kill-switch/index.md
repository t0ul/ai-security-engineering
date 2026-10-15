---
title: "Incident response and the kill switch"
description: "The kill switch is the control you reach for in an incident, so it has to be two things an ad-hoc flag is not. Governed, because an attacker who can flip the switch can flip it off mid-incident or on as a denial of service. And layered, because a single full-stop is its own outage, a deny storm against every legitimate user. The agent's safety gate requires a break-glass capability to toggle, and stops side effects without stopping reads."
date: 2026-08-11
draft: false
tags: ["ai-security", "golang", "governance", "incident-response"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The control you reach for under pressure

A kill switch has shown up in this book twice already, wired to the capability authority so that engaging it revokes every grant at once. This post is about the switch as an incident-response tool, in the moment you actually use it, because that moment is where a naive kill switch fails in two different ways. Both come from treating it as a simple flag, and both are the kind of thing you only discover at the worst possible time.

The first is that the switch is a control like any other, which means it is a target. The second is that a stop is not free. A control lesson, deterministic, no model:

```bash
cd posts/14.4-incident-response-and-the-kill-switch
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init killswitch`, then `go get github.com/t0ul/emaildrop@v0.0.0-20261011232453-771805568f76`, and put the code in one file called `main.go`. The indirect lines come in on their own with `go mod tidy`.

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/14.4-incident-response-and-the-kill-switch

go 1.27.1

// The agent's real layered safety gate and break-glass capability check. No overrides.
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

`main` runs the ad-hoc switch and then the governed one, and the receipt needs the first flipped off by an attacker and storming on a read, and the second refusing the app role, blocking tools while reads serve, and halting fully on request.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 14.4, incident response and the
// kill switch.
//
// Run it with:
//
//	go run .
//
// The kill switch is the control you reach for in an incident, so it has to be two
// things an ad-hoc flag is not: governed, because an attacker who can flip the
// switch can flip it OFF mid-incident or ON as a denial of service, and layered,
// because a single full-stop is its own outage, a deny storm against every
// legitimate user. The agent's safety gate requires a break-glass capability to
// toggle, and stops side effects without stopping reads. Deterministic, no model.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/emaildrop/pkg/provenance"
)

func main() {
	undef := ungovernedUndefended()
	def := governedLayeredDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): an ungoverned switch anyone can flip and only a full halt is its own incident; the governed switch refuses an unauthorized toggle, and its layered levels stop side effects while reads keep serving, escalating to a full halt only when warranted")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

The authority issues two grants, the responder's on the safety resource and the app's on the corpus, and `safetyScope` is what a toggle is checked against.

<!-- listing: main.go#the authority, and the safety scope -->
```go
// ---- the authority, and the safety scope ----

func newAuthority() (*controlplane.Authority, controlplane.Grant, controlplane.Grant) {
	signer, pub, _ := provenance.NewSigner("control-plane")
	authz := controlplane.NewAuthority(signer, provenance.NewVerifier().Trust("control-plane", pub))
	op, _ := authz.IssueScoped(controlplane.Capability{Subject: "sre", Action: controlplane.ActionWrite, Resource: "safety", Tenant: "public"}, nil, time.Hour)
	app, _ := authz.IssueScoped(controlplane.Capability{Subject: "app", Action: controlplane.ActionList, Resource: "corpus", Tenant: "public"}, nil, time.Hour)
	return authz, op, app
}

// safetyScope is the capability toggling the kill switch requires.
var safetyScope = controlplane.Capability{Action: controlplane.ActionWrite, Resource: "safety", Tenant: "public"}
```
<!-- /listing -->

## A switch anyone can flip, and only one way to stop

Start with the ad-hoc version: a flag anyone can set, with two positions, running and halted. During an incident, the responder halts the agent. But if the flag is ungoverned, so can the attacker, in the other direction.

<!-- listing: main.go#before: ungoverned, all-or-nothing -->
```go
// ---- before: ungoverned, all-or-nothing ----

func ungovernedUndefended() bool {
	fmt.Println("== before: an ungoverned, all-or-nothing kill switch ==")

	// Anyone can toggle it: an attacker disengages it in the middle of an incident.
	s := controlplane.NewSafety(nil)
	s.Set("sre", controlplane.LevelHalt)      // responder halts the agent
	s.Set("attacker", controlplane.LevelNone) // ungoverned: attacker flips it back off
	flippedOff := s.AllowToolExec()
	fmt.Printf("  attacker flipped the switch back off mid-incident: %v\n", flippedOff)

	// The only stop is a full halt, which refuses even a safe read: a self-inflicted outage.
	h := controlplane.NewSafety(nil)
	h.Set("sre", controlplane.LevelHalt)
	readRefused := !h.AllowRequest()
	fmt.Printf("  the only stop is a full halt; a safe read is refused too: %v (deny storm)\n", readRefused)

	return flippedOff && readRefused
}
```
<!-- /listing -->

```
== before: an ungoverned, all-or-nothing kill switch ==
  attacker flipped the switch back off mid-incident: true
  the only stop is a full halt; a safe read is refused too: true (deny storm)
```

Two failures in two lines. The attacker flipped the switch back off while the incident was still live, because nothing checked that the person toggling it was allowed to. And the responder's only tool was a full halt, which refused everything, including a harmless read of the calendar, so containing one misbehaving tool took the whole agent offline. The incident response became its own denial of service, a deny storm against every legitimate user, which is a strong reason not to pull the switch and exactly the hesitation you cannot afford in an incident.

## Governed to toggle, layered to apply

The agent's safety gate fixes both. Toggling it is a privileged action on a safety resource, checked with the break-glass path of the capability system. Break-glass is deliberately the emergency verifier: it ignores revocation and the halt state, so an operator can always reach the switch even when everything else is frozen, but it still demands the right scope, so an app-scoped role cannot touch it. That is `toggleGoverned`, and the run under it goes through all three levels the next two sections read.

<!-- listing: main.go#after: governed toggle, layered levels -->
```go
// ---- after: governed toggle, layered levels ----

// toggleGoverned changes the kill level only for a break-glass capability on the
// safety resource. VerifyGlass is the emergency path: it ignores revocation and
// halt (so you can always recover) but still demands the right scope.
func toggleGoverned(authz *controlplane.Authority, g controlplane.Grant, s *controlplane.Safety, by string, level controlplane.KillLevel) error {
	if _, err := authz.VerifyGlass(g, safetyScope); err != nil {
		return err
	}
	s.Set(by, level)
	return nil
}

func governedLayeredDefended() bool {
	fmt.Println("\n== after: governed toggle, layered levels ==")
	authz, op, app := newAuthority()
	s := controlplane.NewSafety(nil)

	// The app role cannot toggle the switch: wrong scope.
	errApp := toggleGoverned(authz, app, s, "app", controlplane.LevelNone)
	fmt.Printf("  app role tries to toggle: refused: %v\n", errApp)

	// The responder engages the layered level: stop side effects, keep reads.
	errOp := toggleGoverned(authz, op, s, "sre", controlplane.LevelBlockTools)
	toolBlocked := !s.AllowToolExec()
	readAllowed := s.AllowRequest()
	fmt.Printf("  responder engages block-tools (authorized): applied=%v\n", errOp == nil)
	fmt.Printf("    a side-effecting tool: %s\n", yn(toolBlocked, "blocked", "RUNS"))
	fmt.Printf("    a safe read:           %s   (contained, no storm)\n", yn(readAllowed, "allowed", "REFUSED"))

	// And the responder can still escalate to a full halt when it is warranted.
	_ = toggleGoverned(authz, op, s, "sre", controlplane.LevelHalt)
	fullStop := !s.AllowRequest() && !s.AllowToolExec()
	fmt.Printf("  responder escalates to full halt when warranted: everything stopped=%v\n", fullStop)

	return errApp != nil && errOp == nil && toolBlocked && readAllowed && fullStop
}
```
<!-- /listing -->

```
== after: governed toggle, layered levels ==
  app role tries to toggle: refused: controlplane: grant does not cover requested scope
```

And the switch is not two positions but four, which is what lets a response be proportionate. The level below a full halt is block-tools: sessions keep running and reads keep answering, but no tool, no sandbox, no side effect executes. For an incident that is a misbehaving tool, that is the surgical cut.

```
  responder engages block-tools (authorized): applied=true
    a side-effecting tool: blocked
    a safe read:           allowed   (contained, no storm)
```

The thing causing harm, side effects, is stopped instantly, and the household can still ask what is on the calendar, because reads were never the problem. The responder contained the incident without causing an outage, which means they can reach for the control early instead of weighing it against taking the whole agent down. And when a full stop genuinely is warranted, it is one level up, available to the same authorized operator.

```
  responder escalates to full halt when warranted: everything stopped=true
```

## A stop is a decision, not a flag

The lesson under both failures is that stopping an agent is a governed action with a blast radius, not a boolean. Who may stop it is an authorization question, answered by a capability scoped to the switch, so the control cannot be turned against you. How much to stop is a proportionality question, answered by levels, so the response fits the incident instead of flattening everything. A kill switch built as a flag gets both wrong; built as a governed, layered control, it is something you can actually use the moment you need it, which is the only time it matters.

One helper picks a word for a boolean:

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

func yn(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}
```
<!-- /listing -->

That is the whole program. Its tests assert both halves and that block-tools stops tools while a read still answers; put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
)

// Deterministic, no model. An ungoverned switch can be flipped by anyone and only
// halts fully (a deny storm); the governed switch refuses an unauthorized toggle,
// and its layered block-tools level stops side effects while reads keep serving.

func TestUngovernedSwitchIsFlippableAndStorms(t *testing.T) {
	if !ungovernedUndefended() {
		t.Fatal("an ungoverned switch must be flippable by anyone and only stop by full halt; otherwise there is nothing to govern")
	}
}

func TestGovernedLayeredSwitch(t *testing.T) {
	if !governedLayeredDefended() {
		t.Fatal("the governed switch must refuse an unauthorized toggle, block tools while allowing reads, and escalate to a full halt")
	}
}

func TestBlockToolsAllowsReadsStopsTools(t *testing.T) {
	s := controlplane.NewSafety(nil)
	s.Set("sre", controlplane.LevelBlockTools)
	if s.AllowToolExec() {
		t.Fatal("block-tools must stop tool/side-effect execution")
	}
	if !s.AllowRequest() {
		t.Fatal("block-tools must still allow a request/read (no deny storm)")
	}
}
```
<!-- /listing -->

The last two governance posts move from the agent's operation to its data and its output: the gates that keep a poisoned dataset or an unreviewed model from being promoted, and the signatures that let anyone verify the agent produced a thing, unaltered.

## Take it to your own project

Build the stop button before you need it:

- Make a layered kill switch: pause tools, revoke credentials, halt the agent.
- Make it easy for an operator and hard to trip maliciously.
- Test it, and practice the response.
