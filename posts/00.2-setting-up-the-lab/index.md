---
title: "Set up your lab: from a bare Mac to a running agent"
description: "The one setup post for the whole series. Starting from a clean Apple Silicon Mac, you install Go and llama.cpp, build the email-to-calendar agent, and watch it turn a real school newsletter into calendar events, all on your own machine. Every later post links back here instead of repeating the setup."
date: 2026-10-02
draft: true
tags: ["ai-security", "golang", "agents", "setup"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop), and the project page is [here](/projects/ai-security-engineering/)._

## Why this post exists

Every other post in the series builds one feature, breaks it, and fixes it. None of them repeat the setup. This is the post they all link back to, so if you ever read "if you have not set up the lab yet, start there," this is where you land.

By the end you will have the agent running on your own machine, reading a real school newsletter and writing out calendar events, with a test that scores the result so "it works" is a number and not a feeling. It all runs locally. Nothing in this post sends your mail, or anyone's, to a cloud service.

I am assuming you have not done much of this before, so I explain each tool the first time it shows up and give you the exact commands to paste. If a step is new to you, that is fine. Go slowly and run each one.

## What you need first

A few things I am taking as given, because sorting them out is its own rabbit hole:

- An **Apple Silicon Mac** (M1 or newer) on macOS 14 or later. The whole project is built and tested there.
- Around **16GB of memory**. The local models are small on purpose, but they still want room.
- **Homebrew** installed, the macOS package manager. If you do not have it, the one-line installer is at [brew.sh](https://brew.sh/). We use it for everything else.
- Roughly **4GB of free disk** for the model files you download near the end.

That is the lot. You do not need Docker, a cloud account, or an API key, and that is kind of the point.

## I. Install the two tools

Two installs cover everything. Paste these one at a time:

```bash
brew install go
brew install llama.cpp
```

**Go** is the language the whole project is written in. I went pure Go for the series because it builds down to a single self-contained binary with a very small dependency tree, and a small dependency tree is a smaller place for supply-chain trouble to hide. That theme comes back later.

**llama.cpp** is the thing that actually runs a language model on your Mac, with no cloud behind it. It takes a model file and serves it over a small local API. We do not need it for the first run (the baseline is deliberately model-free), but installing it now gets the slow download out of the way.

Check that Go is recent enough. You want 1.27 or newer:

```bash
go version
```

```
go version go1.27.1 darwin/arm64
```

## II. Point the model runtime where the agent looks

Homebrew installs a single `llama` command that can serve a model. The project's model launcher looks for that binary at `~/.llama-app/llama` by default, so make a link there:

```bash
mkdir -p ~/.llama-app
ln -sf "$(brew --prefix)/bin/llama" ~/.llama-app/llama
```

Confirm it is the right binary. This should print a short list of commands:

```bash
llama --help
```

```
Usage: llama <command> [options]
Available commands:
  serve           HTTP API server
  cli             Command-line interactive interface
  help            Show available commands
```

If you see `serve` in that list, you are set. (If your llama.cpp is older and only has a `llama-server` command, run `brew upgrade llama.cpp` to get the newer unified one.)

## III. Get the code and build the agent

The series lives in two repositories, and it helps to know which is which before you clone anything. The agent itself, the thing you run, is `t0ul/emaildrop`. The lessons, one folder per post holding that post's attack and its fix, are in `t0ul/ai-security-engineering`. This post is about getting the agent running, so clone that one and step into it:

```bash
git clone https://github.com/t0ul/emaildrop
cd emaildrop
```

Now build the first piece, the part that watches a folder for email and extracts events from it. It is called `emaildrop`:

```bash
CGO_ENABLED=0 go build -o emaildrop ./cmd/emaildrop
```

`CGO_ENABLED=0` tells Go to build without any C compiler involved, which gives you one clean, self-contained binary. The project uses a pure-Go SQLite library specifically so this works, and building without a C toolchain is one less moving part an attacker could lean on. If it compiled without printing anything, it worked, and you now have an `emaildrop` binary in the current folder.

## IV. The first run, no model needed

Here is the part I like, because it shows the agent doing real work before we have touched a language model at all.

The repo ships a folder of sample emails in `testdata/emaildrop/samples/`. These are anonymized copies of real school newsletters, the exact kind of thing I built this for, with the real names and addresses scrubbed out so they are safe to share. (The actual mail I feed it lives in a folder the repo never commits, for obvious reasons.) The agent reads from an `inbox` folder, so make one and drop a sample in:

```bash
mkdir -p drop/inbox
cp testdata/emaildrop/samples/3.txt drop/inbox/newsletter.txt
```

Now run one pass. `EXTRACT_MODE=regex` tells the agent to use its deterministic, rules-based extractor instead of a model. There is no AI in this run at all, which is on purpose: I want a baseline I fully understand before I add something that can be talked into mistakes. `DROP_DIR` points it at the folder you just made, and `--once` means do a single scan and stop:

```bash
EXTRACT_MODE=regex DROP_DIR=./drop ./emaildrop --once
```

You will see a stream of lines marked with a small receipt icon, one per step the agent took, all sharing a trace id. That stream is the agent narrating itself, and it becomes important a few posts from now:

```
🧾 [request] start trace=4a4778fc
🧾 [index] stored trace=4a4778fc
🧾 [event_extractor] start trace=4a4778fc
🧾 [event_extractor] result trace=4a4778fc
🧾 [pipeline] summary_written trace=4a4778fc
🧾 [watcher] handled trace=4a4778fc
```

Then a summary of what it found in that one email:

```json
[
  {
    "trace_id": "4a4778fcfa01213ad60d81edb361b582",
    "source": "newsletter.txt",
    "events": 12,
    "artifacts": [
      "drop/outbox/newsletter.events.ics",
      "drop/outbox/newsletter.items.ics"
    ],
    "warnings": [
      "extractor_mode=regex",
      "9 item(s)",
      "3 action item(s)",
      "PII-scoped: surfaced 0 contact(s)"
    ],
    "moved_to": "20261011-125622.newsletter.txt"
  }
]
```

A quick tour of the folders it just used, because the layout matters later:

- `drop/inbox/` is where mail waits to be read.
- `drop/outbox/` is where it writes its results.
- `drop/processed/` is where it files each email after reading it, under a timestamped name, so nothing gets read twice and the original is kept.

Look in the outbox and you will find an `.ics` file. That is an iCalendar file, the standard format your calendar app understands, and each `VEVENT` block in it is one calendar entry:

```bash
head -17 drop/outbox/newsletter.events.ics
```

```
BEGIN:VCALENDAR
VERSION:2.0
PRODID:-//ai-security-engineering//email-to-calendar//EN
X-WR-CALNAME:Email-to-Calendar
BEGIN:VEVENT
UID:3e72eb2e876b52499a3379042a415131@email-to-calendar
DTSTAMP:20261011T165622Z
X-KIND:event
DTSTART:20260924T083000
SUMMARY:PTA Meeting
LOCATION:Gymnatorium
BEGIN:VALARM
ACTION:DISPLAY
DESCRIPTION:PTA Meeting
TRIGGER:-PT30M
END:VALARM
END:VEVENT
```

Notice what did not happen: nothing was added to a real calendar. The agent wrote a file and stopped. Importing it is a thing you do by hand, on purpose, so a bad extraction can never quietly land on your actual schedule. That human step is a security decision, and it gets its own post.

## V. Prove it with a number

"It pulled some events" is not good enough for a series about getting things right. So the project ships an evaluation harness: a set of sample emails with the correct answers written down by hand, and a scorer that checks the agent against them. The answer key lists the handful of events that really must be caught (the "Save the Dates" in this newsletter), which is why the scorer talks about three events even though the full pass above pulled more date-like lines out of the body.

Run it in the same deterministic mode:

```bash
EXTRACT_MODE=regex go run ./cmd/eval
```

```
=== eval: samples/3.txt ===
gold=3 pred=3 matched=3
precision=1.00 recall=1.00 f1=1.00
  [ok] PTA Meeting            start 2026-09-24T08:30:00==2026-09-24T08:30:00:true  end -==-:true  loc:true
  [ok] Back-to-School Night   start 2026-09-29T17:30:00==2026-09-29T17:30:00:true  end 2026-09-29T20:00:00==2026-09-29T20:00:00:true  loc:true
  [ok] Multicultural Potluck  start 2026-10-23T17:30:00==2026-10-23T17:30:00:true  end 2026-10-23T19:00:00==2026-10-23T19:00:00:true  loc:true
field accuracy: start 3/3  end 3/3  location 3/3
```

That `f1=1.00` is the receipt. F1 is a single score that combines two things: precision, how many of the events it found were real, and recall, how many of the real events it found. A 1.00 means it found every event, invented none, and got the start, end, and location right on all of them. When a later post claims a defense works, it ends on a number like this that you can reproduce by running the same command.

## VI. Wake up the brain

The deterministic agent is genuinely useful, but the series is about the harder, more interesting case: handing the job to a language model, which is where all the security questions start. So let us get the model runtime up. You will not lean on it until the next post, but this readies the lab.

First, download the model files. The project pulls them from their sources and checks each one against a known hash, so you are not running some file a web page handed you:

```bash
go run ./cmd/prepareassets
```

This fetches a few gigabytes (a 1.9GB planner model, a 1.0GB coder model, an 80MB embedding model, plus some files used much later for the isolation posts), so it is a good moment to get a coffee. It only has to happen once.

With the files in place, start the model servers with one command:

```bash
go run ./cmd/modeld
```

```
modeld: starting dual-SLM runtime (Ctrl-C to stop)...
```

`modeld` brings up two small local models at once using the `llama` you installed earlier: a planner on `127.0.0.1:11435` and a coder on `127.0.0.1:11436`, both bound to localhost so nothing off your machine can reach them. It waits for each one to report healthy, then streams their logs. Leave it running in that terminal. In another one, you can check it is up:

```bash
curl -s http://127.0.0.1:11435/health
```

That is the whole lab: the code, a deterministic baseline you can trust, a test that scores it, and a local model runtime ready for the real work. Everything on one Mac, nothing leaving it.

## Conclusion

If you got the `f1=1.00` line, you are in good shape, and that is the milestone that matters here. The next post takes this exact setup and does the first interesting thing with it: points the agent at the model instead of the rules, and runs Attack-Driven Design end to end on one small feature, so the loop we keep talking about stops being abstract. Keep the repo handy, and I will see you there.
