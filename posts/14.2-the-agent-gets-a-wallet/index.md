---
title: "The agent gets a wallet"
description: "The agent is a non-human identity that holds a wallet of short-lived, signed, scoped grants it presents to act. When one leaks you revoke it, and revocation has to take effect now. The classic bug is that it does not: a token checked only for its expiry keeps working after revocation until it times out. The agent's authority consults the revocation list on every check, and the kill switch revokes the whole wallet at once."
date: 2026-07-28
draft: false
tags: ["ai-security", "golang", "governance", "identity"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## An identity that holds credentials

An agent that does anything in the world needs credentials to do it: a token to send mail, a grant to write the calendar, a scope to fetch a URL. It is a non-human identity with a wallet, and like any wallet, the question that matters is not what is in it when everything is fine but what happens the moment one of the cards is stolen. Credentials leak. A grant ends up in a log, a prompt, a crash dump. When that happens, you revoke it, and the entire value of revocation is that it is immediate.

The bug that makes it not immediate is one of the most common in credential systems, and it is quiet, because everything looks like it is working. A control lesson, deterministic, no model:

```bash
cd posts/14.2-the-agent-gets-a-wallet
go run .
```

## Revoked, and still working

The grants the agent carries are bearer tokens: signed, and stamped with an expiry. The tempting way to check one is to confirm it is well-formed and not expired, and honor it if so. Mint a grant, revoke it, and check it that way.

```
== before: a token checked only for expiry ==
  after revocation, the naive check still honors the token: true
```

The token was revoked, and the check honored it anyway, because revoking a credential and checking a credential were two different code paths that never spoke. The grant is still signed and still inside its lifetime, so an expiry check has no reason to refuse it. It will keep working until it times out on its own, which could be an hour or a day of an attacker acting with the agent's full authority, long after you thought you had cut it off.

## Revocation on every check

The fix is that a credential is never valid on its own; it is valid only relative to an authority that can still take it back. The agent verifies every grant through `controlplane.Authority`, which checks the signature and the expiry and the scope, and also, on every single call, whether the subject has been revoked.

```
== after: the authority consults revocation ==
  revoked token verifies: controlplane: grant revoked
  a fresh, un-revoked grant still works:  true
```

The revoked grant is refused the instant it is revoked, not when it expires, because revocation and verification are now the same path: to use a grant is to ask the authority, and the authority remembers what it took back. And revocation is targeted, not a panic button, so a freshly issued grant to a different subject keeps working normally. You cut off the compromised identity without taking the agent down.

## One switch for the whole wallet

Sometimes you do want the panic button, and it is the same mechanism widened. The kill switch from the agency chapter is wired to the authority, so engaging it revokes every grant at once: the same valid token that verified a moment ago is refused the instant the switch is thrown.

```
  kill switch: same grant before=<nil>, after engage=controlplane: grant revoked
```

That is the containment primitive the whole operator story rests on. Revoking a subject answers "this one identity is compromised"; the kill switch answers "stop everything, now", and both reduce to the same question asked at every credential check, is this still allowed. Nothing the agent holds is a standing permission; it is a permission the authority can withdraw between one action and the next, for one identity or for all of them.

The next two posts are the other half of operating safely: when a change does go wrong, rolling it back cleanly and proving it was rolled back, and then the incident itself, the kill switch under the kind of pressure where you find out whether its own controls are as careful as the ones it protects.

## Take it to your own project

Give the agent its own identity, not yours:

- Issue scoped, short-lived, revocable credentials.
- Rotate them, and be able to revoke instantly.
- Never let the agent hold a credential broader than its job.
