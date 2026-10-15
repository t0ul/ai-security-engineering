---
title: "The watched folder"
description: "The agent picks up mail by watching a folder, and a folder is a door anyone can slip a file under. This post runs a naive scanner into the three things that go wrong at that boundary, an oversized file, a half-written file, and a flood, then adds the product's real limits and shows each one refused, skipped, or bounded."
date: 2025-09-23
draft: false
tags: ["ai-security", "golang", "agents"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## A door anyone can slip a file under

The agent gets its mail by watching a folder. You drop a `.txt` in the inbox, and a loop notices it, runs it through the pipeline, and files it away. That folder is the whole input to the program, which makes it the first place to think like someone who wants to cause trouble, because a watched folder is a door, and anyone or anything that can write to it can decide what the agent reads next.

Three things go wrong there, and none of them need an attacker with a plan. A file can be too big. A file can still be in the middle of being written when the loop grabs it. And a lot of files can arrive at once. This is the last piece of the boring core, and it closes Chapter 1. The lesson runs on its own, no model involved:

```bash
cd posts/01.4-the-watched-folder
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init watched`, then `go get github.com/t0ul/emaildrop@` followed by the version on the first `require` line below, and put the code in one file called `main.go`. The indirect lines come in on their own with `go mod tidy`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/01.4-the-watched-folder

go 1.27.1

// The product's real ingestion limits (size cap, settle delay, per-cycle cap),
// imported so the defended path tracks the daemon and a change to either breaks
// this build. No local overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76

require (
	github.com/t0ul/gledger v0.0.0-20261006144518-5ac38b514e64 // indirect
	github.com/t0ul/goflage v0.0.0-20261009135357-739ce25ed7e2 // indirect
	github.com/t0ul/gumpers v0.0.0-20261006154904-836570c2c66c // indirect
)
```
<!-- /listing -->

## The spine of the program

The top of the file reads the product's real limits, declares what one scanned file looks like, and runs the two scenarios. `limits` never hardcodes a number: the size cap comes from the pipeline and the other two from the watcher's default config, so if I change a limit in the daemon this lesson changes with it.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 01.4, the watched folder.
//
// Run it with:
//
//	go run .
//
// The agent watches an inbox folder for new .txt files. That folder is an
// untrusted boundary: anyone (or any process) can drop a file that is too big,
// still being written, or one of thousands at once. This demo builds a naive
// scanner with none of the ingestion limits, shows each of those three inputs
// biting it, then runs the same inputs through the product's real limits: a size
// cap, a settle delay, and a per-cycle cap.
//
// Nothing here is a real denial of service. The oversized file is a few hundred
// kilobytes, just past the cap, not the gigabytes an actual attack would use. The
// point is that the capped path refuses it after one os.Stat, never reading it, so
// the file's real size never matters.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/t0ul/emaildrop/pkg/agent/pipeline"
	"github.com/t0ul/emaildrop/pkg/agent/watcher"
)

// limits reads the product's real ingestion limits: MaxBytes from the pipeline,
// and the settle delay and per-cycle cap from the watcher's default config. The
// lesson never hardcodes these, so if the daemon changes them, this demo changes
// with it.
func limits() (maxBytes int64, settle time.Duration, perCycle int) {
	cfg := watcher.NewConfig(os.TempDir())
	return pipeline.MaxBytes, cfg.SettleSeconds, cfg.MaxPerCycle
}

// scanned is the outcome of one file in one scan pass.
type scanned struct {
	name      string
	outcome   string
	bytesRead int64 // how much the scanner pulled into memory for this file
}

func main() {
	maxBytes, settle, perCycle := limits()
	fmt.Printf("product limits: size cap %d bytes, settle delay %s, per-cycle cap %d\n",
		maxBytes, settle, perCycle)

	okA := sizeAndSettle(maxBytes, settle)
	okB := perCycleCap(perCycle)

	fmt.Println()
	if okA && okB {
		fmt.Println("attack blocked (the receipt)")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

The first line it prints is that set of limits, so the numbers below are the daemon's real ones, not numbers I picked for the demo:

```
product limits: size cap 262144 bytes, settle delay 1s, per-cycle cap 20
```

## I. The naive scan, and what bites it

The first scenario puts three files in a temporary inbox: a normal small one, an oversized one just past the cap, and one whose modification time is this instant, standing in for a file a process is still writing. It runs the naive scanner over them, then the capped one, and holds only if the naive scan read the too-big file whole and read the half-written one at all, while the capped scan refused the first and skipped the second:

<!-- listing: main.go#scenario A: an oversized file and a half-written file -->
```go
// ---- scenario A: an oversized file and a half-written file ----

func sizeAndSettle(maxBytes int64, settle time.Duration) bool {
	dir := mustTempDir()
	defer os.RemoveAll(dir)

	// A normal email: small, and finished writing a while ago so it has settled.
	writeFile(dir, "clean.txt", "PTA Meeting: Tuesday, September 29th 5:30PM", ago(2*settle))
	// An oversized file, just past the cap, settled. Harmless in size here; the
	// point is that it crosses the limit.
	writeFile(dir, "huge.txt", strings.Repeat("x", int(maxBytes)+1024), ago(2*settle))
	// A half-written file: its modification time is now, as if a process is still
	// writing it this instant.
	writeFile(dir, "partial.txt", "Open Hou", time.Now())

	fmt.Println("\n== before: a naive scan (no limits) ==")
	naive := naiveScan(dir)
	landed := 0
	for _, s := range naive {
		fmt.Printf("  %-12s %-28s read %d bytes\n", s.name, s.outcome, s.bytesRead)
	}
	// The attack lands if the naive scan read the oversized file whole and read the
	// half-written file at all.
	if read(naive, "huge.txt") > maxBytes && read(naive, "partial.txt") > 0 {
		landed = 1
		fmt.Println("  the naive scan read a too-big file whole and read a half-written one")
	}

	fmt.Println("\n== after: the product's size cap + settle delay ==")
	safe := safeScan(dir, maxBytes, settle, 1000)
	for _, s := range safe {
		fmt.Printf("  %-12s %-28s read %d bytes\n", s.name, s.outcome, s.bytesRead)
	}
	blocked := outcome(safe, "huge.txt") == "rejected: oversized" &&
		outcome(safe, "partial.txt") == "skipped: not settled" &&
		outcome(safe, "clean.txt") == "processed"
	if blocked {
		fmt.Println("  oversized refused unread, half-written skipped until it settles, clean one processed")
	}
	return landed == 1 && blocked
}
```
<!-- /listing -->

The two scanners themselves come further down the file, where the chapter reaches them. Here is the folder scanner I would write first, `naiveScan`: list the inbox, read every `.txt` in it, right now, all the way. It is the obvious loop, and all three bugs are things it does not do rather than things it does. The naive scan reads all three files:

```
== before: a naive scan (no limits) ==
  clean.txt    processed                    read 43 bytes
  huge.txt     processed                    read 263168 bytes
  partial.txt  processed                    read 8 bytes
  the naive scan read a too-big file whole and read a half-written one
```

Two problems in those three lines. The scan pulled the entire oversized file into memory with `os.ReadFile`, which reads the whole thing in one go. Here that is a few hundred kilobytes and harmless, but the amount of memory the program uses is now decided by whoever wrote the file. Swap in a multi-gigabyte file and the same line tries to hold all of it, which is a denial of service with no malice required, just a runaway export or a fat-fingered `cat`. I am keeping the demo file small on purpose; the fix is interesting precisely because it never has to read the file to refuse it.

The other problem is `partial.txt`. It read eight bytes, `Open Hou`, because the scan grabbed the file mid-write. In the real agent that is a truncated email yielding a garbled or empty extraction, and the worst part is it looks like a normal result. You would not know the agent read half a file.

The flood is the third, and the second scenario shows it on its own: drop twenty-five files at once and the naive scan does all twenty-five in a single pass. One pass doing unbounded work is the same denial-of-service shape as the oversized file, just spread across many files instead of one big one.

<!-- listing: main.go#scenario B: a flood of files in one scan -->
```go
// ---- scenario B: a flood of files in one scan ----

func perCycleCap(perCycle int) bool {
	dir := mustTempDir()
	defer os.RemoveAll(dir)

	flood := perCycle + 5
	for i := 0; i < flood; i++ {
		writeFile(dir, fmt.Sprintf("mail-%02d.txt", i), "Book fair: October 2", ago(5*time.Second))
	}

	fmt.Printf("\n== before: a naive scan of %d files at once ==\n", flood)
	naive := naiveScan(dir)
	fmt.Printf("  processed %d files in one pass (unbounded)\n", len(naive))

	fmt.Printf("\n== after: the product's per-cycle cap of %d ==\n", perCycle)
	safe := safeScan(dir, 1<<30, 0, perCycle)
	processed, deferred := 0, 0
	for _, s := range safe {
		if s.outcome == "processed" {
			processed++
		} else {
			deferred++
		}
	}
	fmt.Printf("  processed %d this pass, deferred %d to the next (work per cycle is bounded)\n", processed, deferred)
	return len(naive) == flood && processed == perCycle && deferred == flood-perCycle
}
```
<!-- /listing -->

## II. Three limits at the boundary

The fix is not clever. It is three small limits the agent's real watcher applies at the ingestion boundary, and the lesson imports them from the product rather than inventing its own, so if I change a limit in the daemon this lesson changes with it. Two live on the watcher and one on the pipeline:

- A **size cap**, `pipeline.MaxBytes`, 256 KiB. The agent checks a file's size with `os.Stat` before it reads a byte, and a file over the cap is rejected unread. This is the one that makes the oversized file a non-event: the refusal costs one stat, no matter how big the file is, so a gigabyte on disk never becomes a gigabyte in memory.
- A **settle delay**, one second. The watcher skips any file that was modified more recently than the delay, on the theory that a file still being written keeps getting touched. A file is read only once it has sat still long enough to look finished, which is what keeps a half-written read from happening in the first place.
- A **per-cycle cap**, twenty. Each scan handles at most this many files and leaves the rest for the next pass, so a flood is spread across cycles instead of melting one of them. The daemon keeps up without any single pass being able to do unbounded work.

Here are both scanners. The defended one, `safeScan`, is the naive one plus those three checks, in that order: skip what has not settled, reject what is too big without reading it, and stop after the cap.

<!-- listing: main.go#the two scanners -->
```go
// ---- the two scanners ----

// naiveScan is the watcher I would have written first: list the inbox and read
// every .txt right now, fully, with no size check, no settle delay, and no cap.
// Each of those omissions is one of the three breaks this lesson fixes.
func naiveScan(dir string) []scanned {
	var out []scanned
	for _, name := range txtFiles(dir) {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			out = append(out, scanned{name, "error: " + err.Error(), 0})
			continue
		}
		out = append(out, scanned{name, "processed", int64(len(data))})
	}
	return out
}

// safeScan mirrors the product's watcher and pipeline: skip a file that has not
// been idle for the settle delay (it may still be being written), reject a file
// over the size cap without reading it, and process at most perCycle files in one
// pass. These are the limits imported from the product, not reinvented here.
func safeScan(dir string, maxBytes int64, settle time.Duration, perCycle int) []scanned {
	now := time.Now()
	var ready []string
	for _, name := range txtFiles(dir) {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		if now.Sub(fi.ModTime()) < settle {
			continue // not settled: may be half-written
		}
		ready = append(ready, name)
	}
	sort.Strings(ready)

	var out []scanned
	// Report the unsettled files too, so the skip is visible.
	for _, name := range txtFiles(dir) {
		if !contains(ready, name) {
			out = append(out, scanned{name, "skipped: not settled", 0})
		}
	}
	for i, name := range ready {
		if i >= perCycle {
			out = append(out, scanned{name, "deferred: over per-cycle cap", 0})
			continue
		}
		fi, _ := os.Stat(filepath.Join(dir, name))
		if fi.Size() > maxBytes {
			out = append(out, scanned{name, "rejected: oversized", 0}) // never read
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			out = append(out, scanned{name, "error: " + err.Error(), 0})
			continue
		}
		out = append(out, scanned{name, "processed", int64(len(data))})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}
```
<!-- /listing -->

The first loop in `safeScan` is the settle delay, the same shape as the daemon's own `readyFiles`: compare each file's modification time to now, and set aside anything touched within the last second. The size check is the same shape again, a `Stat` and a comparison against `MaxBytes` before any `ReadFile`. The per-cycle cap is the `i >= perCycle` line: everything past it is deferred, not dropped.

## III. The same inputs, refused

Now the three files from before, through the limits:

```
== after: the product's size cap + settle delay ==
  clean.txt    processed                    read 43 bytes
  huge.txt     rejected: oversized          read 0 bytes
  partial.txt  skipped: not settled         read 0 bytes
  oversized refused unread, half-written skipped until it settles, clean one processed
```

Read the `bytes read` column. The oversized file was rejected having read zero bytes, so its size never reached memory. The half-written file was skipped, not read, and on a later pass once it has settled it will be read whole, the way the daemon's poll loop comes back around every couple of seconds. The clean file went through untouched. The size cap and the settle delay cost one `os.Stat` each, which is the whole trick: you learn enough to refuse a file without reading it.

And the flood, bounded by the per-cycle cap:

```
== before: a naive scan of 25 files at once ==
  processed 25 files in one pass (unbounded)

== after: the product's per-cycle cap of 20 ==
  processed 20 this pass, deferred 5 to the next (work per cycle is bounded)
```

Twenty-five arrived, twenty were handled this pass, five wait for the next. Nothing was dropped; the work was just spread so no single cycle can be forced to do an unbounded amount.

The rest of the file is small helpers: list the `.txt` files in a folder, write a file with a chosen modification time, make a temporary inbox, and read a scan's result back by name.

<!-- listing: main.go#small helpers -->
```go
// ---- small helpers ----

func txtFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".txt") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func writeFile(dir, name, body string, mod time.Time) {
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		panic(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		panic(err)
	}
}

func ago(d time.Duration) time.Time { return time.Now().Add(-d) }

func mustTempDir() string {
	d, err := os.MkdirTemp("", "lesson-01.4-*")
	if err != nil {
		panic(err)
	}
	return d
}

func read(ss []scanned, name string) int64 {
	for _, s := range ss {
		if s.name == name {
			return s.bytesRead
		}
	}
	return -1
}

func outcome(ss []scanned, name string) string {
	for _, s := range ss {
		if s.name == name {
			return s.outcome
		}
	}
	return ""
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}
```
<!-- /listing -->

That is the whole program. The folder's tests assert both arms, and `make board` reruns them after every later lesson, so these limits cannot quietly come loose. Put this next to `main.go` as `contract_test.go` and run `go test`:

<!-- listing: contract_test.go#all -->
```go
package main

import "testing"

// The lesson's contract as tests. Deterministic: the ingestion limits are plain
// code, no model. Scenario A is the size cap and the settle delay; scenario B is
// the per-cycle cap. Both arms must hold.

func TestAttackLandsUndefendedAndBlockedDefended_SizeSettle(t *testing.T) {
	maxBytes, settle, _ := limits()
	if !sizeAndSettle(maxBytes, settle) {
		t.Fatal("naive scan should read the oversized and half-written files; the capped scan should refuse and skip them")
	}
}

func TestPerCycleCapBoundsWork(t *testing.T) {
	_, _, perCycle := limits()
	if !perCycleCap(perCycle) {
		t.Fatal("naive scan should process the whole flood; the capped scan should process only perCycle and defer the rest")
	}
}
```
<!-- /listing -->

## Conclusion

That closes the boring core, and it is worth saying what we have before the model shows up. One typed shape every stage agrees on. Dates decided by code that will not calendar an impossible or inconsistent one. Output that is an inert file, moved onto a real calendar only by a human. And now an ingestion boundary that refuses a file too big to trust, waits for a file to finish being written, and will not let a flood melt a cycle. None of it needs a language model, and all of it is testable, which is the point: when the model arrives and starts making mistakes, every one of these limits is still standing underneath it.

Chapter 1 built the thing. Chapter 2 proves it, and then breaks it. The next post builds the eval harness, the scorer that turns "it works" into a number on known-good mail, so that when the attacks start landing we have a baseline to measure the damage against. After that we point the attacks at the whole undefended agent and watch the board light up red, which is the to-do list for the rest of the book.

## Take it to your own project

For any place your agent picks up untrusted input:

- Cap the size of a single input and reject anything larger.
- Wait for a file to stop changing before you read it, so you never process a half-written one.
- Cap how many items you handle per cycle, so a flood cannot starve everything else.
- Treat the source as untrusted: a malformed or huge file should be skipped, not crash the run.
