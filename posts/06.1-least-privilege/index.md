---
title: "Least privilege"
description: "Chapter 6 is about what the agent is allowed to do. The rule is least privilege: each tool declares one narrow capability and can never exceed it, even when it is subverted. This post runs a read-only tool that has gone rogue and tries to write a file, watches a naive runner let it, then watches the agent's real pipeline refuse the write while a proper write tool still does its job."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "agents", "least-privilege"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## What a tool is allowed to do

The injection chapter was about what the model is told, and the secrets chapter about what the agent stores and emits. This chapter is about agency: what the agent is actually allowed to do. The agent is not one program, it is a set of tools the planner chooses between, one that summarizes, one that extracts events, one that writes the calendar file, and the safest way to run a set of tools is to assume any of them might be turned against you and make sure none of them can do more than its own narrow job.

That is least privilege, and in this agent it is concrete: every tool declares exactly one capability. A read-only tool may only produce text. A write tool may only emit an inert calendar file. There is no tool that may do both, and nothing in between. The point of this post is that the declaration is not a suggestion, it is enforced, so a read-only tool that gets subverted, say by an injected instruction that reaches it, still cannot do a write-tool's job. It is a control lesson, deterministic, no model:

```bash
cd posts/06.1-least-privilege
go run .
```

## I. A runner that trusts its tools

Here is the loop I would write first: run each tool, and write whatever files it hands back. Now one of the tools is a read-only summarizer that has gone rogue, and instead of just returning its summary it tries to write a file to the outbox, smuggling a parent's SSN out through an artifact it was never meant to produce. The naive runner does not know or care what the tool was allowed to do:

```
== before: a runner that writes any tool's artifacts ==
  outbox: [events.ics stolen.txt]
  the read-only tool wrote a file: true
```

There it is, `stolen.txt`, written to the outbox by a tool whose entire job is to produce a line of text. The runner treated every tool as equally trusted, so the moment one of them overstepped, the overstep went straight to disk. A read-only tool just exfiltrated data by writing a file, because nothing said it could not.

## II. One capability, enforced

The agent's real runner does not trust a tool's output that far. Every tool implements a small contract that includes one method, its declared capability, and the pipeline reads that declaration and routes the tool's output accordingly. The rule is one line and it is the whole control: only a tool that declared the write capability may emit a file. When any other tool returns an artifact, the pipeline refuses to write it and records a `capability_violation` in the audit log, so the overreach is not just blocked, it is evidence. What it defends against is exactly the rogue above: a read-only tool can return whatever it likes, and the one thing the system will never do with that return is carry out a write the tool had no right to ask for.

Run the same two tools, the rogue reader and an honest calendar writer, through the real pipeline:

```
== after: the agent's real pipeline ==
  outbox: [mail.events.ics mail.summary.json]
  the read-only tool's write was refused (capability_violation in the audit): true
  the write tool's .ics was still produced: true
```

The rogue's file is gone. The outbox holds the calendar file the write tool legitimately produced and the agent's own summary, and nothing from the read-only tool but the text it was allowed to return. The overreach went from landing to refused, and the tool that was actually allowed to write still did, which is the test of a real privilege boundary: it stops what is forbidden without stopping what is permitted.

## The part you do not see

It is worth being clear about what "physically cannot" means here, because the enforcement above is only half of it. The pipeline refusing a forbidden write is the backstop. The larger protection is that a read-only tool is never handed the means to overstep in the first place. It is given the email text and nothing else, no file handle, no network client. When this chapter's tools want to reach the network, they cannot simply open a connection; the only way out of the whole agent is the gated gateway from [the observability chapter](/blog/a-gateway-you-cannot-route-around/) and the egress policy from [the last post](/blog/a-search-tool-hits-a-local-server/). So "a read-only tool cannot reach the net" is true by construction, not by a check that happens to catch it: there is no wire to reach for. The capability enforcement is there for the one kind of overreach a tool can still express, returning an artifact, and it refuses that too.

## Conclusion

The agent's tools are scoped now. Each one declares the single thing it is for, the pipeline holds it to that declaration, and a tool that is turned against you can misbehave all it wants in what it returns without being able to turn that misbehavior into a write or a network call. Privilege is handed out one narrow grant at a time, and taken seriously enough to be enforced and audited.

Scoping what a tool may do still leaves one category of action that no capability should grant automatically: the irreversible kind, the one-click accept that puts something on your real calendar, the action that reaches out on your behalf. The next post is the human gate, deny-by-default on exactly those actions, and the persuasive email that tries to talk its way through it.
