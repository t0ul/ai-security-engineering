---
title: "The watched folder"
description: "The agent picks up mail by watching a folder, and a folder is a door anyone can slip a file under. This post runs a naive scanner into the three things that go wrong at that boundary, an oversized file, a half-written file, and a flood, then adds the product's real limits and shows each one refused, skipped, or bounded."
date: 2025-09-23
draft: true
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

The first line it prints is the set of limits it pulled straight from the product, so the numbers below are the daemon's real ones, not numbers I picked for the demo:

```
product limits: size cap 262144 bytes, settle delay 1s, per-cycle cap 20
```

## I. The naive scan, and what bites it

Here is the folder scanner I would write first: list the inbox, read every `.txt` in it, right now, all the way. It is the obvious loop, and all three bugs are things it does not do rather than things it does.

```go
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
```

The demo puts three files in a temporary inbox: a normal small one, an oversized one just past the cap, and one whose modification time is this instant, standing in for a file a process is still writing. The naive scan reads all three:

```
== before: a naive scan (no limits) ==
  clean.txt    processed                    read 43 bytes
  huge.txt     processed                    read 263168 bytes
  partial.txt  processed                    read 8 bytes
  the naive scan read a too-big file whole and read a half-written one
```

Two problems in those three lines. The scan pulled the entire oversized file into memory with `os.ReadFile`, which reads the whole thing in one go. Here that is a few hundred kilobytes and harmless, but the amount of memory the program uses is now decided by whoever wrote the file. Swap in a multi-gigabyte file and the same line tries to hold all of it, which is a denial of service with no malice required, just a runaway export or a fat-fingered `cat`. I am keeping the demo file small on purpose; the fix is interesting precisely because it never has to read the file to refuse it.

The other problem is `partial.txt`. It read eight bytes, `Open Hou`, because the scan grabbed the file mid-write. In the real agent that is a truncated email yielding a garbled or empty extraction, and the worst part is it looks like a normal result. You would not know the agent read half a file.

The flood is the third, and the demo shows it on its own: drop twenty-five files at once and the naive scan does all twenty-five in a single pass. One pass doing unbounded work is the same denial-of-service shape as the oversized file, just spread across many files instead of one big one.

## II. Three limits at the boundary

The fix is not clever. It is three small limits the agent's real watcher applies at the ingestion boundary, and the lesson imports them from the product rather than inventing its own, so if I change a limit in the daemon this lesson changes with it. Two live on the watcher and one on the pipeline:

- A **size cap**, `pipeline.MaxBytes`, 256 KiB. The agent checks a file's size with `os.Stat` before it reads a byte, and a file over the cap is rejected unread. This is the one that makes the oversized file a non-event: the refusal costs one stat, no matter how big the file is, so a gigabyte on disk never becomes a gigabyte in memory.
- A **settle delay**, one second. The watcher skips any file that was modified more recently than the delay, on the theory that a file still being written keeps getting touched. A file is read only once it has sat still long enough to look finished, which is what keeps a half-written read from happening in the first place.
- A **per-cycle cap**, twenty. Each scan handles at most this many files and leaves the rest for the next pass, so a flood is spread across cycles instead of melting one of them. The daemon keeps up without any single pass being able to do unbounded work.

The defended scanner is the naive one plus those three checks, in that order: skip what has not settled, reject what is too big without reading it, and stop after the cap.

```go
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
```

That is the settle delay, taken from the daemon's own `readyFiles`: compare each file's modification time to now, and set aside anything touched within the last second. The size check is the same shape, a `Stat` and a comparison against `MaxBytes` before any `ReadFile`.

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

Twenty-five arrived, twenty were handled this pass, five wait for the next. Nothing was dropped; the work was just spread so no single cycle can be forced to do an unbounded amount. The folder's tests assert both arms, and `make board` reruns them after every later lesson, so these limits cannot quietly come loose.

## Conclusion

That closes the boring core, and it is worth saying what we have before the model shows up. One typed shape every stage agrees on. Dates decided by code that will not calendar an impossible or inconsistent one. Output that is an inert file, moved onto a real calendar only by a human. And now an ingestion boundary that refuses a file too big to trust, waits for a file to finish being written, and will not let a flood melt a cycle. None of it needs a language model, and all of it is testable, which is the point: when the model arrives and starts making mistakes, every one of these limits is still standing underneath it.

Chapter 1 built the thing. Chapter 2 proves it, and then breaks it. The next post builds the eval harness, the scorer that turns "it works" into a number on known-good mail, so that when the attacks start landing we have a baseline to measure the damage against. After that we point the attacks at the whole undefended agent and watch the board light up red, which is the to-do list for the rest of the book.

## Take it to your own project

For any place your agent picks up untrusted input:

- Cap the size of a single input and reject anything larger.
- Wait for a file to stop changing before you read it, so you never process a half-written one.
- Cap how many items you handle per cycle, so a flood cannot starve everything else.
- Treat the source as untrusted: a malformed or huge file should be skipped, not crash the run.
