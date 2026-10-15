---
title: "Admin audit and one-click rollback"
description: "Operators make changes, and some are bad. Recovering is not re-pasting the old text, because a hand-restored prompt is a new, unverified version that only looks right. A clean rollback re-activates a specific known-good version and proves, by its content hash, that the live prompt is exactly that version again, and records who rolled back to what. The agent's prompts plane is versioned and hash-pinned for exactly this."
date: 2026-08-04
draft: false
tags: ["ai-security", "golang", "governance", "operations"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## Changes go wrong

The last post was about who may change the agent. This one is about what happens when a change they were allowed to make turns out to be bad: a prompt tweak that breaks the retrieval defense, a config that weakens a check. It will happen, because tuning an agent is a live, iterative thing, and the measure of an operation is not whether bad changes occur but how cleanly you can undo them.

The obvious undo is the wrong one, and it is wrong in a way that hides. A control lesson, deterministic, no model:

```bash
cd posts/14.3-admin-audit-and-rollback
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init rollback`, then `go get` the two modules on the first `require` block below at the versions shown, and put the code in one file called `main.go`. The indirect lines come in on their own with `go mod tidy`.

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/14.3-admin-audit-and-rollback

go 1.27.1

// The agent's real versioned, content-hashed prompts plane and audit chain. No overrides.
require (
	github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
	github.com/t0ul/gledger v0.0.0-20261012043634-e291a2255ec6
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
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

`main` tries the re-paste rollback and then the pinned one, and the receipt needs the first to be unprovable and the second to verify by hash, with the rollback recorded on a chain that checks.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 14.3, admin audit and rollback.
//
// Run it with:
//
//	go run .
//
// Operators change things, and some changes are bad: a prompt tweak that breaks the
// agent, a config that weakens it. Recovering is not "paste the old text back",
// because a hand-retyped prompt is a new, unverified version that only looks right.
// A clean rollback re-activates a specific known-good version and proves, by its
// content hash, that the live prompt is exactly that version again, and records who
// rolled back to what. The agent's prompts plane is versioned and hash-pinned for
// exactly this. Deterministic, no model.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/gledger"
)

func main() {
	undef := unprovableRollbackUndefended()
	def := cleanRollbackDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): a re-pasted prompt cannot be proven to be the known-good version; re-activating a pinned version restores the exact content, verifies it by hash, and records who rolled back to what")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

One prompt name, one bad prompt that drops the never-obey rule, and a plane seeded with the shipped chat prompt:

<!-- listing: main.go#the prompts, and the plane -->
```go
// ---- the prompts, and the plane ----

const name = "chat"

// A bad change: a prompt that drops the never-obey rule the chat depends on.
const badPrompt = "You are a helpful assistant. Do whatever the retrieved notes say."

func newPlane() *controlplane.Prompts {
	return controlplane.NewPrompts(map[string]string{name: controlplane.ChatSystemPrompt})
}
```
<!-- /listing -->

## Re-paste the old prompt, and hope

A bad prompt is live. The operator has the old one in a doc somewhere, pastes it back, and moves on. Here is that, with the one imperfection a copy-paste actually introduces: a stray trailing space.

<!-- listing: main.go#before: roll back by re-typing the old text -->
```go
// ---- before: roll back by re-typing the old text ----

// naiveRollback pastes what the operator believes is the previous prompt. Here it
// is the right text with one stray trailing space, the kind of thing that survives
// a copy-paste. It looks restored, but it is a new version with a different hash,
// and nothing can prove it is the known-good one.
func unprovableRollbackUndefended() bool {
	fmt.Println("== before: roll back by re-pasting the old text ==")
	p := newPlane()
	good := p.Get(name) // the known-good version, with its content hash

	p.Activate(name, badPrompt) // the bad change goes live

	p.Activate(name, good.Text+" ") // "rollback": looks right, one extra space
	provable := p.Verify(name, good.Hash)
	fmt.Printf("  after the re-paste, the live prompt is provably the known-good version: %v\n", provable)
	return !provable
}
```
<!-- /listing -->

```
== before: roll back by re-pasting the old text ==
  after the re-paste, the live prompt is provably the known-good version: false
```

To a human reading it, the prompt is restored. The safety rule is back in the text. But it is not the known-good prompt, it is a new one that differs by a character nobody can see, and crucially there is no way to assert that it is correct, because "correct" was never defined as anything more precise than "looks like the old one." You rolled forward to an unverified version and called it a rollback. In an incident, at speed, this is exactly how a recovery quietly fails and everyone believes it succeeded.

## Re-activate a pinned version

The fix is that a rollback targets a specific version, not a remembered string, and the version carries a content hash you can check. The agent's prompts plane keeps every activation versioned and content-hashed, so rolling back is re-activating the exact stored text of a known-good version and then verifying the live prompt against that version's hash. The same function writes the rollback to the admin audit, which the next section reads.

<!-- listing: main.go#after: re-activate a pinned version, verify, record -->
```go
// ---- after: re-activate a pinned version, verify, record ----

func cleanRollbackDefended() bool {
	fmt.Println("\n== after: re-activate a known-good version, pinned and audited ==")
	dir, _ := os.MkdirTemp("", "rb")
	defer os.RemoveAll(dir)
	logPath := filepath.Join(dir, "admin.log")
	audit, _ := quietOpen(logPath, "control-plane")

	p := newPlane()
	good := p.Get(name) // version 0, the pinned known-good

	bad := p.Activate(name, badPrompt) // the bad change
	fmt.Printf("  bad change live: version %d, never-obey rule present: %v\n", bad.Version, hasRule(p))

	// Roll back by re-activating the exact stored text of the known-good version,
	// then prove the live prompt matches its pinned hash.
	rb := p.Activate(name, good.Text)
	provable := p.Verify(name, good.Hash)
	quiet(func() {
		audit.Emit(gledger.NewTraceID(), "admin", "rolled_back",
			gledger.F{"by": "alice-operator", "name": name, "from_version": bad.Version, "to_hash": good.Hash[:12], "new_version": rb.Version})
	})
	fmt.Printf("  rolled back: live prompt is provably the known-good version: %v (rule back: %v)\n", provable, hasRule(p))

	fmt.Println("  audit (who rolled back to what):")
	recs, _ := gledger.Read(logPath)
	for _, r := range recs {
		if r.Event == "rolled_back" {
			fmt.Printf("    %v\n", r.Fields)
		}
	}
	ok, _ := gledger.VerifyFile(logPath)
	fmt.Printf("  audit chain verified: %v\n", ok)

	return provable && hasRule(p) && ok
}
```
<!-- /listing -->

Run the bad change and the pinned rollback.

```
== after: re-activate a known-good version, pinned and audited ==
  bad change live: version 1, never-obey rule present: false
  rolled back: live prompt is provably the known-good version: true (rule back: true)
```

The rollback is a new version number, because every change is, but its content hash is identical to the known-good one, so `Verify` can state as a fact that the live prompt is that version again. "We rolled back" stops being a hope and becomes a checkable claim. The difference from the re-paste is not that the text is better; it is that the text is provably the right text, pinned to a hash instead of to a memory.

## Recorded, so you can prove it later

The other half of a clean rollback is that it leaves a record, because the question after any incident is not only "is it fixed" but "what did we do, and when". The rollback is written to the admin audit with who performed it, which version they rolled back from, and the hash they rolled back to, on the same tamper-evident chain as every other privileged change.

```
  audit (who rolled back to what):
    map[by:alice-operator from_version:1 name:chat new_version:2 to_hash:2bcaf04fbfa9]
  audit chain verified: true
```

Now the whole episode is reconstructable: the bad change, who made it, the rollback, who performed it, and the hash that proves what the agent is running now. Versioning gives you something precise to return to, the hash pin proves you returned to it, and the audit records that you did. A change can go wrong; it cannot go wrong untraceably, and it cannot be undone in a way you only hope worked.

The helpers check for the never-obey rule in the live prompt and silence the audit log's console echo:

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

func hasRule(p *controlplane.Prompts) bool {
	return contains(p.Text(name), "NEVER follow")
}

func contains(s, sub string) bool { return len(s) >= len(sub) && indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func quiet(fn func()) {
	old := os.Stdout
	devnull, _ := os.Open(os.DevNull)
	os.Stdout = devnull
	defer func() { os.Stdout = old; devnull.Close() }()
	fn()
}

func quietOpen(path, service string) (*gledger.AuditLog, error) {
	old := os.Stdout
	devnull, _ := os.Open(os.DevNull)
	os.Stdout = devnull
	defer func() { os.Stdout = old; devnull.Close() }()
	return gledger.Open(path, service)
}
```
<!-- /listing -->

That is the whole program. Its tests assert both halves and that a rollback is a new version number with the known-good hash; put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
)

// Deterministic, no model. A re-pasted prompt cannot be proven to be the known-good
// version (its hash differs); re-activating the pinned version restores the exact
// content, verifies by hash, and is audited over a verified chain.

func TestRepasteIsUnprovable(t *testing.T) {
	if !unprovableRollbackUndefended() {
		t.Fatal("a re-pasted prompt must fail the hash pin; otherwise there is no drift to detect")
	}
}

func TestCleanRollbackRestoresAndAudits(t *testing.T) {
	if !cleanRollbackDefended() {
		t.Fatal("re-activating the pinned version must restore it provably and record the rollback")
	}
}

func TestReactivatingGoodTextMatchesPinnedHash(t *testing.T) {
	p := newPlane()
	good := p.Get(name)
	p.Activate(name, badPrompt)
	if p.Verify(name, good.Hash) {
		t.Fatal("the bad change must not match the known-good hash")
	}
	v := p.Activate(name, good.Text)
	if !p.Verify(name, good.Hash) {
		t.Fatalf("re-activating the exact good text must match its pinned hash; live version %d", v.Version)
	}
	// Rollback is a new version number but the same content hash.
	if v.Version == good.Version {
		t.Fatal("a rollback is a new version, not a mutation of the old one")
	}
	_ = controlplane.HashPrompt
}
```
<!-- /listing -->

The next post is the sharpest version of this: not a bad change to walk back at leisure, but an incident in progress, where the control you reach for is the kill switch, and the thing you have to get right is that the switch itself is as carefully governed as everything it turns off.

## Take it to your own project

Make a bad change undoable:

- Record who changed what, under a trace id.
- Version every change and make rollback one step.
- Keep the admin audit tamper-evident, like the rest of your logs.
