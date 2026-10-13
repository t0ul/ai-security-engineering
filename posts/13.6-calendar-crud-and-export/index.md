---
title: "Calendar CRUD and export"
description: "The danger in this feature is not an attacker. It is the agent doing the same safe thing twice: a watcher re-reads an email, a request is retried, an export runs again, and the same event lands on the calendar a second time. A naive write fires on every call. The agent routes each write through a durable exactly-once ledger keyed by the event's content, so a replay returns the first result and never writes again."
date: 2026-07-07
draft: true
tags: ["ai-security", "golang", "agents", "reliability"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The quiet failure

Most of this book has been about a hostile email. This feature fails without one. Writing events to the calendar and exporting them is the agent's one durable side effect, the thing that shows up on your actual phone, and the way it goes wrong is the most ordinary bug there is: it happens twice. The folder watcher restarts and re-reads an email it already processed. A request times out and the client retries. An export runs a second time. Every one of those is correct behavior, a safe operation repeated, and every one of them, done naively, puts a second copy of the same event on your calendar.

It is not a security attack in the usual sense, but it is the same discipline, because a side effect that fires more than once is exactly how a retried refund becomes a double refund. Getting "exactly once" right is a reliability property and a safety property at the same time. A control lesson, deterministic, no model:

```bash
cd posts/13.6-calendar-crud-and-export
go run .
```

## Write on every pass, duplicate on every pass

Here is the naive write: an event comes in, put it on the calendar. Now process the same email twice, the way a restarted watcher would.

```
== before: a write that fires on every pass ==
  calendar after two ingests of one email: [Multicultural Potluck Multicultural Potluck]
  duplicate written: true
```

Two Potlucks. The agent did nothing wrong on either pass; each time it saw the event and added it, which is what a write does. The duplicate is not a bug in the write, it is the absence of any memory that this particular write already happened.

## Key the write, remember it happened

That memory is the fix, and it has a standard shape: idempotency. Give each write a key derived from the event's content, the title, the date, the kind, so the same event always produces the same key no matter how many times it is re-ingested. Then route the write through a durable ledger that records which keys it has already done. The agent uses `pkg/durable` for this, the same exactly-once ledger that keeps a retried action from firing its side effect twice.

```go
l.Do("import", "calendar-write", eventKey(e), func() (string, error) {
	return addEvent(e.Title) // the side effect, run at most once per key
})
```

Process the same email twice through it.

```
== after: the durable exactly-once ledger ==
  calendar after two ingests of one email: [Multicultural Potluck]
  written exactly once: true (the replay was recognized and skipped: true)
```

One Potluck. The first pass ran the write and recorded its key; the second pass looked the key up, saw it was already done, and returned the first result instead of writing again. The watcher can re-read that email a hundred times and the calendar does not change after the first, because the ledger turns "do this" into "do this if it has not been done", keyed by what the thing actually is.

## Exactly once is a safety property

It is worth ending the feature on why this belongs in a security book at all. The export is dup-free for the same reason: it keys each event and skips the ones already exported, so importing into a real calendar happens exactly once. And the ledger underneath is the same primitive that will matter far more in the reliability chapters, where the side effect is not adding a calendar line but spending money or sending a message. "At most once" for a dangerous action is the same engineering as "exactly once" for a calendar write: a durable record of what has already happened, keyed so a replay cannot pretend to be new. The agent is allowed to retry anything; it is not allowed to let a retry count twice.

That is the last of the agent's own features. The one surface left in the capstone is the one people touch directly, the chat box over everything the agent knows, and its attack brings the whole book full circle: a prompt injection, arriving this time through the corpus the chat retrieves.

## Take it to your own project

Guard the writes, not just the reads:

- Put least privilege and the human gate on writes and deletes.
- Sanitize exported fields and keep the export inert.
- Attack create, update, delete, and export each on its own.
