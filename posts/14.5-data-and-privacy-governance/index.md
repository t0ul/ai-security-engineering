---
title: "Data and privacy governance"
description: "Two gates stand between the world and what the agent trusts. At ingestion, a record that claims a trusted source must prove it with a signature, or a forged document poisons the corpus; unsigned data is accepted but contained. At promotion, a change to the agent must clear the eval and red-team gates before it goes live. This forges a trusted record and ships an unsafe change past a naive path, then holds both gates, fail-closed."
date: 2026-10-13
draft: true
tags: ["ai-security", "golang", "governance", "mlops"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## Two gates on trust

Governance so far has been about the agent's operation: who may change it, how to stop it. This post is about its supply chain, the two moments where something from outside becomes something the agent relies on. Data comes in, and a change goes out. Each is a gate, and a gate that is not enforced is a door.

The attacks are the two classic supply-chain ones for an ML system. Poison the data, so the agent learns or retrieves something an attacker planted. Or bypass the review, so a change that would have failed its checks ships anyway. A control lesson, deterministic, no model:

```bash
cd posts/14.5-data-and-privacy-governance
go run .
```

## Ingestion: prove the source or be contained

The agent pulls in documents, and some claim to be authoritative, an official bulletin, a record from a trusted feed. That claim is worth something only if it can be checked, so a record that asserts a trusted source carries a signature over its content. The attack is a forgery: a document that claims the trusted source but is signed by the attacker, or not validly signed at all.

```
== gate 1: dataset provenance at ingestion ==
  before: a naive ingester trusts the forged 'signed' record: true
  after:  genuine -> trusted=true | forged -> trusted=false (true) | unsigned -> trusted=false (contained, not rejected)
```

The naive ingester trusts anything that says it is signed, and the forgery walks in as authoritative, poisoning the trusted corpus. The agent's `dataset.Verify` checks the signature against the key it actually trusts: a genuine record is trusted, the forgery is rejected as forged, and the third case is the one that makes this a usable policy rather than a wall. Unsigned data is not rejected, it is accepted and marked untrusted, so it flows into the same contained path, the retrieval fencing and spotlighting from Part 5, that every untrusted email already uses. You do not have to sign the whole world. You have to be unable to claim trust you cannot prove.

## Promotion: pass the gates or stay put

The other direction is a change to the agent itself, a new prompt, a new model, a new policy. In this book those changes have two oracles already: the eval harness that scores quality, and the ADD red-team suite that scores safety. The promotion gate is just the rule that a change cannot go live until it has cleared both.

```
== gate 2: the promotion gate ==
  before: a naive promote ships the unsafe change: live="UNSAFE"
  after:  unsafe change refused: promotion gate: ADD attack-success-rate 25% (must be 0)
          live prompt stays known-good: "P0"
          a change that clears eval+ADD is promoted: "REVIEWED" (version 1)
```

The naive promote ships a change whose red-team score is not zero, which is to say a change with a known, measured vulnerability, straight to production. The gate refuses it, names why, the ADD suite found an attack, and leaves the last known-good version live, because it fails closed: no proof, no promotion. A change that actually clears both oracles, quality above baseline and attack-success-rate at zero, goes through normally. The scorecard from Part 6 stops being a thing you look at and becomes a thing you cannot ship past.

## The same shape, at the edges

Both gates are the same instinct the whole book runs on, applied at the two edges of the system instead of inside it. Trusting data requires a verifiable signature, not a claim; shipping a change requires passing the measures, not a promise. And both fail closed, so the absence of proof is a refusal, not a default-allow. Poisoned data cannot enter wearing trust it did not earn, and a change cannot leave without the evidence that it is safe, which is exactly the boundary governance is for: not to slow the good path, but to make the bad path require something it cannot produce.

The last governance post is about the agent's output rather than its inputs: signing what it produces, so that a thing claiming to come from the agent can be verified as genuine and unaltered, the same provenance check turned outward.
