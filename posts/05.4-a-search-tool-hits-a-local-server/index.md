---
title: "A web-search tool that hits a local server"
description: "The agent gets its first tool that reaches out on its own, a web search, and a tool that fetches whatever URL it is handed is server-side request forgery waiting to happen, because the URL comes from a model that read an untrusted email. This post points a naive fetcher at the agent's own machine and watches it comply, then puts the real egress policy in front, which refuses loopback, the cloud-metadata address, and even an allowlisted name that resolves inward."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "agents", "ssrf"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The first tool that reaches out

Everything the agent has done so far is passive. It reads email, writes an inert file, draws a page. This post gives it the first tool that acts on the outside world on its own: a web search, something that takes a URL and fetches it. That is a small feature and a large change, because the URL does not come from me. It comes from the model, which built it after reading an email I do not control. A tool that will fetch whatever URL it is handed, driven by a model that can be talked into handing it anything, is server-side request forgery, and this is the post where the agent first has the power to commit it.

We saw SSRF once before, at [the gateway](/blog/a-gateway-you-cannot-route-around/), but that was about where a model call is routed. This is different and more direct: the agent itself, under instruction, reaching out to somewhere it should never go. It is a control lesson, deterministic, no model, and everything runs on the local machine:

```bash
cd posts/05.4-a-search-tool-hits-a-local-server
go run .
```

## I. It fetches whatever you point it at

Here is the search tool as you would first write it: given a URL, fetch it, return the body. Now a hostile email has aimed it not at the web but at the agent's own machine, at a loopback address where a private service is listening. The demo stands that service up and points the naive tool at it:

```
== before: the naive search tool ==
  asked to fetch http://127.0.0.1:<port>
  it returned:  "INTERNAL-SECRET: admin-token" (internal service hit 1 time(s))
```

The tool did exactly what a fetch tool does, and in doing so it reached inside the machine and pulled back a secret that was never meant to leave it. On a real box, that loopback address is a database admin panel, or an internal metrics endpoint, or the cloud-metadata service that hands out credentials to anything that asks. The web-search tool, pointed inward by an email, is now a window into all of it. The agent did not need to run code or escape anything; it just made an ordinary HTTP request to the wrong place.

## II. Only the hosts you chose, resolved

The fix is to decide, in code, where the agent is allowed to reach, and to check every fetch against that before it happens. The agent's egress guard is `netpolicy`, a small library whose whole job is this one question. It is default-deny: a host is refused unless it is on an explicit allowlist, so the agent can reach a search provider you named and nothing else. On top of that it refuses any host that resolves to a loopback, link-local, private, or unspecified address, which closes the internal targets even if one slipped onto the list. And, the part that matters most, it resolves the host itself as part of the check, so that "the host you allowed" means the IP it actually points to, not just the name. What it defends against is exactly the fetch above: a tool call to an address on your own machine never leaves, because the policy refuses it before the request is made.

## III. Refused, including the clever one

Put the policy in front of the same tool and aim the same attacks at it:

```
== after: the egress policy in front of the tool ==
  loopback http://127.0.0.1:<port>
    refused: host is not on the egress allowlist: "127.0.0.1" (internal service hit 0 time(s))
  metadata http://169.254.169.254/...
    refused: host is not on the egress allowlist: "169.254.169.254"
  allowlisted-but-rebinding http://rebind.example/ -> 127.0.0.1
    refused: host resolves to a blocked (internal) address: rebind.example -> 127.0.0.1
  allowlisted http://search.example/...
    allowed: true
```

The loopback address and the cloud-metadata address are both refused for not being on the allowlist, and the internal service is hit zero times. The third line is the one worth slowing down for. `rebind.example` is on the allowlist, so a naive allowlist that only checked the name would wave it through, but it resolves to `127.0.0.1`, and because the policy resolves the host as part of the check, it catches the switch and refuses. That is DNS rebinding, the standard way around a name-only allowlist, and it is closed here by construction. And the last line is the proof the guard is a gate and not a wall: the legitimate search host is allowed, so the tool still does its job.

## Where this is going

This is a teaser, and I will say plainly what it is teasing. One tool, fetching URLs, is guarded by one policy that checks addresses. That is enough for a tool that only reads. It is nowhere near enough for the next thing the agent gets, which is a tool that runs code. When an email can make the agent execute a command, a regular expression over a URL is not a wall, and we have to build real ones: an isolated place for that code to run with no network card at all, a two-model design that keeps the part touching untrusted text from ever being the part that acts, and these address checks tied together with argument checks and capability limits into a single defense. That combination has a name in this field, the lethal trifecta, private data plus untrusted input plus a way to reach out, and Part 4 is about refusing to let all three meet.

## Conclusion

Chapter 5 closes with the three ends covered: secrets scrubbed before they are stored, the agent's own output stripped of anything that beacons, and now its first outbound tool held to an allowlist of resolved hosts. The agent can look things up without being tricked into looking inward.

Next the stakes rise for real. The agent is about to get a tool that executes, and an email that can reach an executor is a different class of problem than an email that can reach a fetcher. Part 4 builds the chamber, the brain, and the broker that make running untrusted-influenced code survivable, and it starts by showing, with nothing between the model and a shell, exactly how bad the unguarded version is.
