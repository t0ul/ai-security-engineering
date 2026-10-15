---
title: "The agent gets a wallet"
description: "The agent is a non-human identity that holds a wallet of short-lived, signed, scoped grants it presents to act. When one leaks you revoke it, and revocation has to take effect now. The classic bug is that it does not: a token checked only for its expiry keeps working after revocation until it times out. The agent's authority consults the revocation list on every check, and the kill switch revokes the whole wallet at once."
date: 2026-07-28
draft: false
tags: ["ai-security", "golang", "governance", "identity"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## An identity that holds credentials

An agent that does anything in the world needs credentials to do it: a token to send mail, a grant to write the calendar, a scope to fetch a URL. It is a non-human identity with a wallet, and like any wallet, the question that matters is not what is in it when everything is fine but what happens the moment one of the cards is stolen. Credentials leak. A grant ends up in a log, a prompt, a crash dump. When that happens, you revoke it, and the entire value of revocation is that it is immediate.

The bug that makes it not immediate is one of the most common in credential systems, and it is quiet, because everything looks like it is working. A control lesson, deterministic, no model:

```bash
cd posts/14.2-the-agent-gets-a-wallet
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init wallet`, then `go get github.com/t0ul/emaildrop@v0.0.0-20261011232453-771805568f76`, and put the code in one file called `main.go`. The indirect lines come in on their own with `go mod tidy`.

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/14.2-the-agent-gets-a-wallet

go 1.27.1

// The agent's real capability authority (scoped grants, revocation, kill switch). No overrides.
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

`main` runs the naive check and then the authority's check, and the receipt needs the revoked token honored the first time and refused the second, with a fresh grant still working and the kill switch refusing everything.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 14.2, the agent gets a wallet.
//
// Run it with:
//
//	go run .
//
// The agent is a non-human identity that holds credentials to act: short-lived,
// signed, scoped grants it presents to send email, fetch a URL, write a calendar.
// A wallet of them. When one leaks, revocation has to mean something immediately,
// and the classic bug is that it does not: a bearer token that only gets its expiry
// checked keeps working after it is revoked, until it times out on its own. The
// agent's authority consults a revocation list on every check, and the kill switch
// revokes the whole wallet at once. Deterministic, no model.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/emaildrop/pkg/provenance"
)

func main() {
	undef := revokedStillWorksUndefended()
	def := revocationHoldsDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): an expiry-only check honors a revoked token until it times out; the agent's authority refuses a revoked grant the instant it is revoked, still honors a fresh one, and the kill switch revokes the whole wallet at once")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

One capability is checked throughout, permission to send email, and `newAuthority` builds the signing authority and a one-line issuer of hour-long grants to a named subject.

<!-- listing: main.go#the capability, and the authority -->
```go
// ---- the capability, and the authority ----

// want is the scope the agent's grant is checked against: permission to send email.
var want = controlplane.Capability{Action: controlplane.ActionExport, Resource: "email", Tenant: "public"}

func newAuthority() (*controlplane.Authority, func(string) controlplane.Grant) {
	signer, pub, _ := provenance.NewSigner("control-plane")
	authz := controlplane.NewAuthority(signer, provenance.NewVerifier().Trust("control-plane", pub))
	issue := func(subject string) controlplane.Grant {
		g, _ := authz.IssueScoped(controlplane.Capability{Subject: subject, Action: controlplane.ActionExport, Resource: "email", Tenant: "public"}, nil, time.Hour)
		return g
	}
	return authz, issue
}
```
<!-- /listing -->

## Revoked, and still working

The grants the agent carries are bearer tokens: signed, and stamped with an expiry. The tempting way to check one is to confirm it is well-formed and not expired, and honor it if so. Mint a grant, revoke it, and check it that way.

<!-- listing: main.go#before: a bearer token checked only for expiry -->
```go
// ---- before: a bearer token checked only for expiry ----

// naiveCheck is the mistake: a grant is honored as long as it has not expired.
// Nothing consults whether it was revoked, so revocation does nothing until the
// token times out on its own, an hour, a day, however long it was minted for.
func naiveCheck(g controlplane.Grant) bool {
	return time.Now().UTC().Before(g.Expires)
}

func revokedStillWorksUndefended() bool {
	fmt.Println("== before: a token checked only for expiry ==")
	authz, issue := newAuthority()
	g := issue("agent")
	authz.Revoke("agent") // the credential leaked; we revoke it right now
	stillWorks := naiveCheck(g)
	fmt.Printf("  after revocation, the naive check still honors the token: %v\n", stillWorks)
	return stillWorks
}
```
<!-- /listing -->

```
== before: a token checked only for expiry ==
  after revocation, the naive check still honors the token: true
```

The token was revoked, and the check honored it anyway, because revoking a credential and checking a credential were two different code paths that never spoke. The grant is still signed and still inside its lifetime, so an expiry check has no reason to refuse it. It will keep working until it times out on its own, which could be an hour or a day of an attacker acting with the agent's full authority, long after you thought you had cut it off.

## Revocation on every check

The fix is that a credential is never valid on its own; it is valid only relative to an authority that can still take it back. The agent verifies every grant through `controlplane.Authority`, which checks the signature and the expiry and the scope, and also, on every single call, whether the subject has been revoked. The same function then flips the kill switch on a third grant, which the next section reads.

<!-- listing: main.go#after: revocation consulted on every verify -->
```go
// ---- after: revocation consulted on every verify ----

func revocationHoldsDefended() bool {
	fmt.Println("\n== after: the authority consults revocation ==")
	authz, issue := newAuthority()

	g := issue("agent")
	authz.Revoke("agent")
	_, errRevoked := authz.Verify(g, want)
	fmt.Printf("  revoked token verifies: %v\n", errRevoked)

	// Revocation is targeted, not a blanket denial: a fresh grant still works.
	fresh := issue("agent-2")
	_, errFresh := authz.Verify(fresh, want)
	fmt.Printf("  a fresh, un-revoked grant still works:  %v\n", errFresh == nil)

	// The kill switch revokes the whole wallet at once: even a valid grant is
	// refused while the agent is halted.
	killed := false
	authz.Halted = func() bool { return killed }
	good := issue("agent-3")
	_, before := authz.Verify(good, want)
	killed = true
	_, after := authz.Verify(good, want)
	fmt.Printf("  kill switch: same grant before=%v, after engage=%v\n", before, after)

	return errors.Is(errRevoked, controlplane.ErrGrantRevoked) &&
		errFresh == nil &&
		before == nil && errors.Is(after, controlplane.ErrGrantRevoked)
}
```
<!-- /listing -->

```
== after: the authority consults revocation ==
  revoked token verifies: controlplane: grant revoked
  a fresh, un-revoked grant still works:  true
```

The revoked grant is refused the instant it is revoked, not when it expires, because revocation and verification are now the same path: to use a grant is to ask the authority, and the authority remembers what it took back. And revocation is targeted, not a panic button, so a freshly issued grant to a different subject keeps working normally. You cut off the compromised identity without taking the agent down.

## One switch for the whole wallet

Sometimes you do want the panic button, and it is the same mechanism widened. The kill switch from the agency chapter is wired to the authority, so engaging it revokes every grant at once: the same valid token that verified a moment ago is refused the instant the switch is thrown.

```
  kill switch: same grant before=<nil>, after engage=controlplane: grant revoked
```

That is the containment primitive the whole operator story rests on. Revoking a subject answers "this one identity is compromised"; the kill switch answers "stop everything, now", and both reduce to the same question asked at every credential check, is this still allowed. Nothing the agent holds is a standing permission; it is a permission the authority can withdraw between one action and the next, for one identity or for all of them.

That is the whole program. Its tests assert both halves and that a reinstated subject's grant verifies again; put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"errors"
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
)

// Deterministic, no model. An expiry-only check honors a revoked token; the real
// authority refuses it the instant it is revoked, still honors a fresh grant, and
// the kill switch revokes every grant at once.

func TestRevokedTokenStillWorksUndefended(t *testing.T) {
	if !revokedStillWorksUndefended() {
		t.Fatal("an expiry-only check must still honor a revoked token; otherwise there is nothing to fix")
	}
}

func TestRevocationAndKillSwitchHold(t *testing.T) {
	if !revocationHoldsDefended() {
		t.Fatal("the authority must refuse a revoked grant, honor a fresh one, and refuse everything under the kill switch")
	}
}

func TestReinstateRestores(t *testing.T) {
	authz, issue := newAuthority()
	g := issue("agent")
	authz.Revoke("agent")
	if _, err := authz.Verify(g, want); !errors.Is(err, controlplane.ErrGrantRevoked) {
		t.Fatalf("a revoked grant must be refused, got %v", err)
	}
	authz.Reinstate("agent")
	if _, err := authz.Verify(g, want); err != nil {
		t.Fatalf("a reinstated subject's grant must verify again, got %v", err)
	}
}
```
<!-- /listing -->

The next two posts are the other half of operating safely: when a change does go wrong, rolling it back cleanly and proving it was rolled back, and then the incident itself, the kill switch under the kind of pressure where you find out whether its own controls are as careful as the ones it protects.

## Take it to your own project

Give the agent its own identity, not yours:

- Issue scoped, short-lived, revocable credentials.
- Rotate them, and be able to revoke instantly.
- Never let the agent hold a credential broader than its job.
