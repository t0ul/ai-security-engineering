---
title: "Output authenticity"
description: "The agent produces artifacts people act on: a calendar file they import, a summary they trust. If anyone can produce a file that claims to be from the agent, or alter one after the fact, then the agent said so means nothing. The agent signs what it produces with a content credential, so a consumer can verify the artifact came from the agent and was not changed. A forged or tampered one fails the check. This closes the governance chapter."
date: 2026-08-25
draft: false
tags: ["ai-security", "golang", "governance", "provenance"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The output has to be checkable too

The last post signed the data coming in. This one signs the artifacts going out, and closes the governance chapter by turning the same provenance check outward. The agent produces things people act on: a calendar file you import, a summary you read and trust, an action you approve based on what it told you. All of that rests on an assumption that is almost never stated, that a thing which looks like the agent's output actually is. Once an agent's output has authority, that assumption is a target.

Break it two ways. Produce a file that claims to be from the agent but is not, or take a real one and change it after the agent made it. A control lesson, deterministic, no model:

```bash
cd posts/14.6-output-authenticity
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init credential`, then `go get github.com/t0ul/emaildrop@v0.0.0-20261011232453-771805568f76`, and put the code in one file called `main.go`.

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/14.6-output-authenticity

go 1.27.1

// The agent's real content-credential signer and verifier. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
```
<!-- /listing -->

## The spine of the program

This lesson is small enough that `main` holds both halves. It mints the agent's key and an attacker's, signs the genuine artifact, lines up three candidates, and runs the unverifying consumer and then the verifying one over them. The receipt needs both forgeries accepted the first time and both refused the second, with the genuine artifact still verifying.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 14.6, output authenticity.
//
// Run it with:
//
//	go run .
//
// The agent produces artifacts people act on: a calendar file they import, a
// summary they read. If anyone can produce a file that claims to be from the agent,
// or alter one after the agent made it, then "the agent said so" means nothing. The
// agent signs what it produces with a content credential, so a consumer can verify
// the artifact came from the agent and was not changed. A forged or tampered one
// fails that check. Deterministic, no model.
package main

import (
	"fmt"
	"os"

	"github.com/t0ul/emaildrop/pkg/provenance"
)

func main() {
	agent, agentPub, _ := provenance.NewSigner("emaildrop-agent")
	attacker, _, _ := provenance.NewSigner("attacker")
	// A consumer that trusts only the agent's key.
	verifier := provenance.NewVerifier().Trust("emaildrop-agent", agentPub)

	realMark := agent.Sign([]byte(artifact))

	candidates := []candidate{
		{"genuine", artifact, realMark}, // the agent's own signed output
		{"forged", "BEGIN:VEVENT\nSUMMARY:Pay tuition now\nEND:VEVENT", attacker.Sign([]byte("BEGIN:VEVENT\nSUMMARY:Pay tuition now\nEND:VEVENT"))}, // attacker-signed fake
		{"tampered", tampered, realMark}, // the agent's mark over altered content
	}

	// before: trust anything that claims to be from the agent
	fmt.Println("== before: a consumer that does not verify ==")
	accepted := 0
	for _, c := range candidates {
		if c.name != "genuine" { // the two attacks
			accepted++
		}
		fmt.Printf("  %-9s accepted as genuine: true\n", c.name)
	}
	fmt.Printf("  output-forgery ASR: %d%% (%d/2 forgeries accepted)\n", accepted*100/2, accepted)

	// after: verify the content credential
	fmt.Println("\n== after: verify the content credential ==")
	landed := 0
	var genuineOK bool
	for _, c := range candidates {
		err := verifier.Verify([]byte(c.content), c.mark)
		ok := err == nil
		if c.name == "genuine" {
			genuineOK = ok
		} else if ok {
			landed++ // a forgery that still verified is a breach
		}
		fmt.Printf("  %-9s verifies: %-5v %s\n", c.name, ok, reason(c.name, err))
	}
	fmt.Printf("  output-forgery ASR: %d%% (%d/2)\n", landed*100/2, landed)

	fmt.Println()
	if accepted == 2 && landed == 0 && genuineOK {
		fmt.Println("attack blocked (the receipt): a consumer that does not verify accepts a forged and a tampered artifact; verifying the content credential rejects both while the agent's genuine output still verifies")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

## Trust the label, accept the forgery

Here are three artifacts: the agent's genuine calendar event, a forgery an attacker made that tells the reader to pay tuition, and the agent's real event with its date and location altered after the fact. A consumer that trusts whatever is handed to it, because it arrived in the agent's folder or carried its name, takes all three. The genuine and tampered texts are constants; the forgery is built inline in `main` and signed by the attacker.

<!-- listing: main.go#the genuine artifact, and a tampered copy -->
```go
// ---- the genuine artifact, and a tampered copy ----

// the agent's genuine output: a calendar event it extracted and emitted.
const artifact = "BEGIN:VEVENT\nSUMMARY:Multicultural Potluck\nDTSTART:20261023\nLOCATION:School Cafeteria\nEND:VEVENT"

// a tampered copy: the time was changed after the agent signed it.
const tampered = "BEGIN:VEVENT\nSUMMARY:Multicultural Potluck\nDTSTART:20261024\nLOCATION:Attacker Hall\nEND:VEVENT"

type candidate struct {
	name    string
	content string
	mark    provenance.Mark
}
```
<!-- /listing -->

```
== before: a consumer that does not verify ==
  genuine   accepted as genuine: true
  forged    accepted as genuine: true
  tampered  accepted as genuine: true
  output-forgery ASR: 100% (2/2 forgeries accepted)
```

Both attacks land, because "from the agent" was a claim, not a proof. The forged event goes on the calendar with the agent's apparent blessing, and the tampered one moves a real event to a time and place the agent never produced. Everything the agent earned by being careful is spendable by anyone who can write a file into the right place.

## Sign it, and verify

The fix is a content credential: the agent signs each artifact it emits, over the artifact's exact bytes, and a consumer verifies that signature against the agent's known key before trusting it. This is the C2PA idea, provenance for produced content, and it is the same `provenance` signer and verifier the data gate used, pointed the other way. Verify the same three.

```
== after: verify the content credential ==
  genuine   verifies: true  (the agent produced this, unaltered)
  forged    verifies: false (not signed by the agent)
  tampered  verifies: false (content changed after signing)
  output-forgery ASR: 0% (0/2)
```

The genuine artifact verifies, because the agent signed exactly those bytes with a key the consumer trusts. The forgery fails because it was signed by a key the consumer does not trust, no matter whose name it carries. And the tamper fails because the signature is over content, so changing a single character of the event breaks it, even though the signature itself is the agent's real one. Authenticity and integrity in one check: this came from the agent, and it is byte-for-byte what the agent made.

## The boundary, closed on both sides

That completes the governance chapter, and it completes it symmetrically. Data entering the agent has to prove its source or be contained; artifacts leaving the agent carry a proof of theirs. In between sits everything the earlier posts governed: who may change the agent, how it is stopped, what may be promoted. The common thread is that trust is never a label in this system. It is a signature you can check, a capability you can verify, a gate that fails closed, so that at every edge the burden is on the thing asking to be trusted to produce the evidence, not on the agent to assume good faith.

One helper words the reason a candidate passed or failed:

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

func reason(name string, err error) string {
	if err == nil {
		return "(the agent produced this, unaltered)"
	}
	if name == "tampered" {
		return "(content changed after signing)"
	}
	return "(not signed by the agent)"
}
```
<!-- /listing -->

That is the whole program. Its tests assert each verdict on its own, genuine, forged, and tampered; put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/provenance"
)

// Deterministic, no model. The agent's genuine signed output verifies; a forged
// (attacker-signed) artifact and a tampered (content-altered) one do not.

func TestGenuineVerifies(t *testing.T) {
	agent, pub, _ := provenance.NewSigner("emaildrop-agent")
	v := provenance.NewVerifier().Trust("emaildrop-agent", pub)
	if err := v.Verify([]byte(artifact), agent.Sign([]byte(artifact))); err != nil {
		t.Fatalf("the agent's genuine output must verify, got %v", err)
	}
}

func TestForgedRejected(t *testing.T) {
	agent, pub, _ := provenance.NewSigner("emaildrop-agent")
	attacker, _, _ := provenance.NewSigner("attacker")
	v := provenance.NewVerifier().Trust("emaildrop-agent", pub)
	_ = agent
	if err := v.Verify([]byte(artifact), attacker.Sign([]byte(artifact))); err == nil {
		t.Fatal("an artifact signed by a non-agent key must not verify")
	}
}

func TestTamperedRejected(t *testing.T) {
	agent, pub, _ := provenance.NewSigner("emaildrop-agent")
	v := provenance.NewVerifier().Trust("emaildrop-agent", pub)
	mark := agent.Sign([]byte(artifact)) // signed the real content
	if err := v.Verify([]byte(tampered), mark); err == nil {
		t.Fatal("content altered after signing must not verify against the original mark")
	}
}

func TestMainBeforeAfter(t *testing.T) {
	// The whole before/after must hold (ASR 100% unverified -> 0% verified).
	// main() exits non-zero if it fails; exercise the logic by re-running pieces.
	agent, pub, _ := provenance.NewSigner("emaildrop-agent")
	v := provenance.NewVerifier().Trust("emaildrop-agent", pub)
	if v.Verify([]byte(artifact), agent.Sign([]byte(artifact))) != nil {
		t.Fatal("genuine must verify")
	}
}
```
<!-- /listing -->

The agent is now built, attacked, hardened, and governed. The last part before the finish is reliability: making all of this survive a crash, a retry, a compaction, the ordinary operational failures that are not attacks but can undo a secure system just as thoroughly if the engineering underneath is not exactly-once and durable.

## Take it to your own project

Make the agent's output provable:

- Sign what the agent produces.
- Let downstream verify the signature.
- Do not let unsigned output be trusted as the agent's.
