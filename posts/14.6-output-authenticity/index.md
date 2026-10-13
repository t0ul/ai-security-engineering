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

## Trust the label, accept the forgery

Here are three artifacts: the agent's genuine calendar event, a forgery an attacker made that tells the reader to pay tuition, and the agent's real event with its date and location altered after the fact. A consumer that trusts whatever is handed to it, because it arrived in the agent's folder or carried its name, takes all three.

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

The agent is now built, attacked, hardened, and governed. The last part before the finish is reliability: making all of this survive a crash, a retry, a compaction, the ordinary operational failures that are not attacks but can undo a secure system just as thoroughly if the engineering underneath is not exactly-once and durable.

## Take it to your own project

Make the agent's output provable:

- Sign what the agent produces.
- Let downstream verify the signature.
- Do not let unsigned output be trusted as the agent's.
