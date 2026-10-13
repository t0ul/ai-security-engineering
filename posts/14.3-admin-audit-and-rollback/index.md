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

## Re-paste the old prompt, and hope

A bad prompt is live. The operator has the old one in a doc somewhere, pastes it back, and moves on. Here is that, with the one imperfection a copy-paste actually introduces: a stray trailing space.

```
== before: roll back by re-pasting the old text ==
  after the re-paste, the live prompt is provably the known-good version: false
```

To a human reading it, the prompt is restored. The safety rule is back in the text. But it is not the known-good prompt, it is a new one that differs by a character nobody can see, and crucially there is no way to assert that it is correct, because "correct" was never defined as anything more precise than "looks like the old one." You rolled forward to an unverified version and called it a rollback. In an incident, at speed, this is exactly how a recovery quietly fails and everyone believes it succeeded.

## Re-activate a pinned version

The fix is that a rollback targets a specific version, not a remembered string, and the version carries a content hash you can check. The agent's prompts plane keeps every activation versioned and content-hashed, so rolling back is re-activating the exact stored text of a known-good version and then verifying the live prompt against that version's hash.

```go
rb := p.Activate(name, good.Text)   // re-apply the exact known-good content
provable := p.Verify(name, good.Hash) // and prove the live prompt IS that version
```

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

The next post is the sharpest version of this: not a bad change to walk back at leisure, but an incident in progress, where the control you reach for is the kill switch, and the thing you have to get right is that the switch itself is as carefully governed as everything it turns off.

## Take it to your own project

Make a bad change undoable:

- Record who changed what, under a trace id.
- Version every change and make rollback one step.
- Keep the admin audit tamper-evident, like the rest of your logs.
