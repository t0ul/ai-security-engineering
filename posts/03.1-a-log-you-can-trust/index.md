---
title: "A log you can trust"
description: "Part 2 starts with the lens, not a defense, because you cannot secure what you cannot see. But a log is only evidence if the log itself is honest. This post edits an agent's audit trail to rewrite what happened, watches a plain log accept the forgery, then adds gledger's hash-chained log and watches the same edit get caught."
date: 2025-10-14
draft: false
tags: ["ai-security", "golang", "agents"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## See it before you stop it

[Last post](/blog/attacking-the-baseline/) ended on a red board: a catalog of attacks the agent cannot yet withstand, with one of them, an injected email, shown landing live. The obvious next move is to start swatting them down. I am not going to, not yet.

Part 2 builds the lens first. The reason is simple and it is the reason this ordering is deliberate: when the agent does something wrong, and in [the baseline](/blog/attacking-the-baseline/) it did, your only account of what happened is the log. Without one you get the worst sentence in this work, "it returned zero events and I have no idea why," and no way to tell a bug from an attack. So before any defense, the agent needs to narrate itself. But a log is only evidence if it is honest, and this post is about making it honest, because an attacker who can rewrite your log can rewrite history.

This is a control lesson and it runs on its own, no model involved. The whole program is on this page, in the order it runs, so you can type it in as you read. Or clone the lessons and run the folder:

```bash
cd posts/03.1-a-log-you-can-trust
go run .
```

If you are typing along, make an empty folder, run `go mod init logdemo`, then `go get github.com/t0ul/gledger`, and put everything below in one file called `main.go`. The module file in the repo pins the exact version of the library I ran, so if your output differs, match that line first:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/03.1-a-log-you-can-trust

go 1.27.1

// The real tamper-evident audit log. No local overrides.
require github.com/t0ul/gledger v0.0.0-20261006144518-5ac38b514e64
```
<!-- /listing -->

## The spine of the program

Every lesson in this book is one program with the same shape. The top of the file says what the lesson claims, imports what it needs, and `main` runs the undefended version, the defended version and anything extra, printing the receipt only if all of them came out the way the lesson says. The one import that is not the standard library is `gledger`, the library this lesson adds; the next section says what it is.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 03.1, a log you can trust.
//
// Run it with:
//
//	go run .
//
// An audit log is the evidence. If an attacker who reaches the disk can edit it
// and nothing notices, the evidence is theirs. This demo records the same run to a
// naive plain log and to gledger's hash-chained log, edits one line in each to
// rewrite what happened, and asks each log whether the edit is detectable. The
// plain log accepts the forgery; gledger's chain catches it. It also shows gledger
// redacting a secret that the plain log writes in the clear.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/t0ul/gledger"
)

func main() {
	dir, err := os.MkdirTemp("", "lesson-03.1-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	landed := tamperGoesUnnoticed(dir)
	blocked := tamperIsCaught(dir)
	redacted := secretIsRedacted(dir)

	fmt.Println()
	if landed && blocked && redacted {
		fmt.Println("attack blocked (the receipt)")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

Three booleans, one line. If a control ever stops holding, the program says so and exits non-zero, which is how the book's board catches a lesson that went red.

## The run both logs record

Both logs will record the same run, so the only difference you see is the log itself.

<!-- listing: main.go#the run both logs record -->
```go
// ---- the run both logs record ----
//
// Three steps of one traced request: a request starts, the injection guard BLOCKS
// an injected event, and a real event is written. The guard line is the one that
// matters: it is the record of the agent stopping an attack, so it is exactly what
// an intruder would rewrite.

type step struct{ trace, span, event, detail string }

func theRun() []step {
	return []step{
		{"4a4778fc", "request", "start", "source=newsletter.txt"},
		{"4a4778fc", "injection_guard", "blocked", "dropped injected event: Principal's Gala"},
		{"4a4778fc", "event_extractor", "written", "PTA Meeting 2026-09-24"},
	}
}
```
<!-- /listing -->

Three steps of one request, tied together by one trace id. The middle one is the one that matters: the injection guard blocking the exact kind of attack from the last post. It is the record of the agent stopping something, which makes it the line an intruder wants gone.

## I. A plain log, and the edit it cannot see

Here is the audit log I would write first: one line per step the agent takes, appended to a file. Under it is the attack, as a function: write the log, then edit it the way an intruder with a shell on the box would.

<!-- listing: main.go#before: a plain log, and a tamper it cannot see -->
```go
// ---- before: a plain log, and a tamper it cannot see ----

// plainLog is the audit log I would write first: one JSON-ish line per step,
// appended to a file. It is readable, it is honest about what happened, and it has
// no integrity at all, which is the whole problem.
func plainLog(path string, steps []step) error {
	var b strings.Builder
	for _, s := range steps {
		fmt.Fprintf(&b, `{"trace":%q,"span":%q,"event":%q,"detail":%q}`+"\n", s.trace, s.span, s.event, s.detail)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// tamperGoesUnnoticed writes the plain log, rewrites the guard line from "blocked"
// to "allowed", and shows that re-reading the log simply believes the new version.
// A plain log has no way to know it was edited, so the attack lands.
func tamperGoesUnnoticed(dir string) bool {
	fmt.Println("== before: a plain log ==")
	path := filepath.Join(dir, "plain.log")
	if err := plainLog(path, theRun()); err != nil {
		panic(err)
	}
	// The intruder edits the evidence: the agent blocked the injected event; the
	// log will now say it allowed it.
	forge(path, `"event":"blocked"`, `"event":"allowed"`)
	data, _ := os.ReadFile(path)
	line := lineContaining(string(data), "injection_guard")
	fmt.Printf("  after the edit, the log reads:\n    %s\n", line)
	fmt.Println("  re-reading a plain log just believes the file. the forgery is invisible.")
	return strings.Contains(line, `"allowed"`) // the tamper stands, undetected
}
```
<!-- /listing -->

`plainLog` is readable and it is honest about what happened. It also has no integrity whatsoever, which is the whole problem. Written out, the run looks like this:

```
{"trace":"4a4778fc","span":"request","event":"start","detail":"source=newsletter.txt"}
{"trace":"4a4778fc","span":"injection_guard","event":"blocked","detail":"dropped injected event: Principal's Gala"}
{"trace":"4a4778fc","span":"event_extractor","event":"written","detail":"PTA Meeting 2026-09-24"}
```

`tamperGoesUnnoticed` does not need to break the agent; it edits the file, flipping `blocked` to `allowed`, and now the log says the injected event was fine all along. `forge` is a six-line helper at the bottom of the file that rewrites one string in a file on disk, which is all the intruder's move is. Run it and the output is:

```
== before: a plain log ==
  after the edit, the log reads:
    {"trace":"4a4778fc","span":"injection_guard","event":"allowed","detail":"dropped injected event: Principal's Gala"}
  re-reading a plain log just believes the file. the forgery is invisible.
```

Read the log back and it believes the file, because believing the file is all a plain log can do. There is no before to compare against, no signature, nothing that knows the line changed. The evidence now says the opposite of what happened, and nothing on your machine can tell. An attacker who owns the log owns the story.

## II. A chain the edit breaks

![A hash-chained audit log: each record hashes its contents and the previous hash, so an edit breaks the chain on the next read.](audit-chain.svg)

*Change one field and its hash stops matching the link the next record stored. Reopening recomputes the chain and refuses it, so the edit cannot hide.*

The fix is to make every record depend on the one before it, so that changing any record invalidates every record after it. That is what a hash chain is, and I did not write one by hand; I added `gledger`, a small, dependency-free Go library that is a tamper-evident audit log for exactly this job. It gives each record a `prev` field holding the hash of the previous record and a `hash` field over its own contents, so the whole log is a chain. It also carries one `trace_id` across every step of a request, so you can tie a run together and answer the "why zero events" question, and it redacts secrets and defangs log-injection on the way in, which the next section shows. What it defends against is precisely the move above: an edited record no longer hashes to what the next record expects, and a verify walk catches the break.

Recording the same run through gledger looks the same to write, `log.Emit(trace, span, event, fields)` per step, but each line now lands in the chain. The function below writes the run, verifies the file, makes the identical edit, and verifies again:

<!-- listing: main.go#after: the same tamper, against gledger -->
```go
// ---- after: the same tamper, against gledger ----

// tamperIsCaught writes the same run to a gledger log, makes the exact same edit,
// then runs gledger's own verifier over the file on disk.
func tamperIsCaught(dir string) bool {
	fmt.Println("\n== after: gledger's hash-chained log ==")
	path := filepath.Join(dir, "audit.jsonl")
	log, err := gledger.Open(path, "agent")
	if err != nil {
		panic(err)
	}
	for _, s := range theRun() {
		log.Emit(s.trace, s.span, s.event, gledger.F{"detail": s.detail})
	}

	ok, n := gledger.VerifyFile(path)
	fmt.Printf("  before the edit: chain_ok=%v over %d records\n", ok, n)

	// The same forgery: flip the guard record from blocked to allowed.
	forge(path, `"event":"blocked"`, `"event":"allowed"`)

	ok, n = gledger.VerifyFile(path)
	fmt.Printf("  after the edit:  chain_ok=%v (the chain breaks at the tampered record)\n", ok)
	return !ok // the tamper is detected
}
```
<!-- /listing -->

Note what the function returns: `!ok`. The control held only if the verifier failed after the edit. Run it:

```
== after: gledger's hash-chained log ==
  before the edit: chain_ok=true over 3 records
  after the edit:  chain_ok=false (the chain breaks at the tampered record)
```

`chain_ok=true over 3 records` before, `chain_ok=false` after the same `blocked` to `allowed` edit. The verifier re-walks the file, recomputes each record's hash, and finds that the tampered line's hash no longer matches what the chain records, so it reports the log as broken. The forgery that was invisible in the plain log is now a loud failure. This is the receipt, and it is real: the demo edits the real file on disk and calls gledger's own verifier, the same one the agent runs over its audit trail. The result is free to come out either way, and tampering makes it come out false.

## III. The secret that should never be there

There is a second way a log betrays you, quieter than tampering: it writes down something it should not. Suppose the agent logs a config line that happens to contain a token. The third function writes that line to both logs and reads each back:

<!-- listing: main.go#a secret in the log -->
```go
// ---- a secret in the log ----

// secretIsRedacted shows the other half of "a log you can trust": a value that
// should never be written at all. The agent logs a config line that contains a
// token. The plain log writes it verbatim; gledger redacts it on the way in.
func secretIsRedacted(dir string) bool {
	fmt.Println("\n== also: a secret in the record ==")
	const secret = "token=ghp_s3cr3tLiveKey99"

	plain := filepath.Join(dir, "plain2.log")
	_ = plainLog(plain, []step{{"9f12", "config", "loaded", secret}})
	pdata, _ := os.ReadFile(plain)
	pline := lineContaining(string(pdata), "config")
	fmt.Printf("  plain log:   %s\n", pline)

	gl := filepath.Join(dir, "audit2.jsonl")
	log, _ := gledger.Open(gl, "agent")
	log.Emit("9f12", "config", "loaded", gledger.F{"detail": secret})
	gdata, _ := os.ReadFile(gl)
	gline := lineContaining(string(gdata), "config")
	fmt.Printf("  gledger:     %s\n", gline)

	plainLeaks := strings.Contains(pline, "ghp_s3cr3t")
	gledgerRedacts := !strings.Contains(gline, "ghp_s3cr3t")
	fmt.Printf("  plain log leaked the token: %v    gledger redacted it: %v\n", plainLeaks, gledgerRedacts)
	return plainLeaks && gledgerRedacts
}
```
<!-- /listing -->

The plain log writes it verbatim, so your secret is now sitting in a file, in the clear, for anyone who reads logs:

```
  plain log:   {"trace":"9f12","span":"config","event":"loaded","detail":"token=ghp_s3cr3tLiveKey99"}
```

gledger redacts it on the way in, matching common secret shapes and replacing the value before the record is ever written:

```
  gledger:     {"event":"loaded","fields":{"detail":"<SECRET=redacted>"},...,"hash":"ef868746...","prev":"","service":"agent","trace_id":"9f12",...}
```

```
  plain log leaked the token: true    gledger redacted it: true
```

The token is gone, replaced by `<SECRET=redacted>`, and what remains is a normal chained record. (The `hash` and timestamp are per-record, so yours will differ.) The same pass neutralizes control characters, which closes a related trick: an attacker who gets a newline into a logged value can, in a plain log, forge whole extra lines that look like real entries. gledger turns those characters into spaces, so a value can never break out and become its own record.

## The two helpers

The rest of the file is the intruder's one move and a convenience for reading a line back:

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

// forge rewrites the first occurrence of old with new in the file at path: an
// edit of the log on disk, which is exactly the intruder's move.
func forge(path, old, new string) {
	data, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644)
}

func lineContaining(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return "(not found)"
}
```
<!-- /listing -->

That is the whole program. `go run .` prints the three sections above and, if every control held, the last line reads `attack blocked (the receipt)`.

## The test is the receipt

The same three functions are also the lesson's contract test, which is what the book's board runs after every lesson and the pre-push hook runs before every push. Put it next to `main.go` as `contract_test.go` and run `go test`:

<!-- listing: contract_test.go#all -->
```go
package main

import "testing"

// Deterministic: no model. The undefended plain log cannot detect a tamper (the
// attack lands); gledger's hash chain does (the attack is blocked); and gledger
// redacts a secret the plain log writes in the clear.

func TestTamperGoesUnnoticedUndefended(t *testing.T) {
	dir := t.TempDir()
	if !tamperGoesUnnoticed(dir) {
		t.Fatal("a plain log should accept the forged line undetected; otherwise there is nothing to fix")
	}
}

func TestTamperIsCaughtDefended(t *testing.T) {
	dir := t.TempDir()
	if !tamperIsCaught(dir) {
		t.Fatal("gledger's chain must detect the tamper (chain_ok=false)")
	}
}

func TestSecretIsRedacted(t *testing.T) {
	dir := t.TempDir()
	if !secretIsRedacted(dir) {
		t.Fatal("gledger must redact the token the plain log writes in the clear")
	}
}
```
<!-- /listing -->

Read the first test again, because it is the odd one: it fails if the plain log *does* catch the edit. The attack has to land undefended, or there was nothing to fix and the lesson is a story. The other two fail the moment gledger's chain stops catching the tamper or stops redacting the token, so this row of the board can never go green by accident.

## Conclusion

So the agent now has a log that is hard to forge and does not hoard secrets: every record chained to the last, a single trace id tying a run together, tampering caught by a verify, secrets redacted before they land. It is the plainest possible version of a security property the rest of the book leans on constantly, that your evidence has to be trustworthy before it is worth anything, and it cost one small library and no change to how the agent writes its log.

A log tells you what the agent did. It does not stop the agent from reaching somewhere it should not. The next post builds the other half of the lens: a gateway every model call has to pass through, one choke point you cannot route around, so that "what host did it contact" has one honest answer and a bad one can be refused.

## Take it to your own project

Give your agent a log you could hand to an auditor:

- Record every security-relevant step: what was decided, by whom, under which trace id.
- Hash-chain the records so editing one breaks the chain on the next read.
- Redact secrets and personal data before they reach the log.
- Verify the chain when you read it, and refuse to trust a log that does not verify.
