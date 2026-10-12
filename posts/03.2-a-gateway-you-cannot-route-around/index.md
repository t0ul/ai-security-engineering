---
title: "A gateway you cannot route around"
description: "A model call should have exactly one way out, through a choke point you control. This post builds the naive proxy first, lets it be steered at an internal service and flooded with no limit, then puts gouncer in front: an allowlist of model names, a body cap, and a rate cap, so an unknown host is a 400, a burst is a 429, and an oversized body is a 413."
date: 2026-10-12
draft: true
tags: ["ai-security", "golang", "agents"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## One way out

[Last post](/blog/a-log-you-can-trust/) gave the agent a log it can trust, so it can tell you what it did. This post gives it the other half of the lens: one door that every model call has to go through. The reason is the same observability instinct. If the agent can open a connection to anywhere, then "what did it talk to" has no single answer, and worse, the answer can be chosen by someone who is not you. A choke point you control turns that into one honest, auditable fact, and it is the place to enforce the first rules.

Two things go wrong at that door, and a naive version gets both wrong by leaving them out. It can be pointed at a host it should never reach, which is server-side request forgery. And it can be hammered with no limit, which for a paid model is not just a crash, it is a bill. This is a control lesson, and it runs on its own:

```bash
cd posts/03.2-a-gateway-you-cannot-route-around
go run .
```

Everything in it runs on throwaway local servers, and the flood is a bounded burst of ten requests. I am not going to hand you a command that hammers a real service, and you should be suspicious of anyone who does.

## I. A proxy the caller can aim

Here is the model proxy I would write first. It reads where to send the call from the request and forwards it there:

```go
func naiveProxy() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstream := extractUpstream(body) // the caller's chosen host, trusted
		resp, _ := client.Post(upstream+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}
```

It works, and that one line, trusting the caller's chosen host, is the hole. The demo stands up a real backend the agent is supposed to use and, separately, an internal service it must never reach through a model call, one that answers with a secret. Then it sends the proxy a request that names the internal service as the upstream:

```
== SSRF: a model call aimed at an internal service ==
  naive proxy:  HTTP 200, internal service hit 1 time(s)  <- SSRF: reached the internal host and leaked its secret
```

The proxy did exactly what it was told. It fetched the internal host and handed its secret back, 200, no complaint. On a real machine that internal host is the cloud metadata endpoint handing out credentials, or an admin service on localhost, and the thing steering the proxy is an instruction in an email the model read. A call that was supposed to reach a model reached inside the network instead.

## II. A door that only knows model names

The fix is to take the choice of host away from the caller entirely. I put `gouncer` in front, a small zero-dependency Go library that is a security gateway for the OpenAI-compatible API that local and hosted models speak. Its whole idea is in one sentence: a caller names a logical model, never a host, and the gateway maps allowed model names to their upstreams. There is no field a request can set to change where the call goes. An unknown model is refused, so server-side request forgery is not mitigated, it is removed, because the lever the attack needs does not exist. It also caps bodies, limits rate and concurrency per client, and carries one trace id end to end into the gledger log from the last post, so every call is one auditable record through one door.

Configuring it is the allowlist and the caps, nothing more:

```go
gouncer.New(gouncer.Config{
	Routes:        map[string]gouncer.Route{"planner": {Upstream: backendURL}},
	MaxBodyBytes:  512,
	RatePerMinute: 60,
	RateBurst:     3,
})
```

`planner` is the one model this gateway will route, and it goes to the one upstream I named. Send the same attack, a request naming the internal service, and the model name it carries is not on the allowlist:

```
  gouncer:      HTTP 400 (model not allowed), internal service hit 0 time(s)
```

A 400, and the internal service was hit zero times. The attacker could not even express the request, because the gateway's vocabulary is model names and `internal` is not one of them. The host is simply not something a caller gets to say.

## III. The flood and the fat request

The other door, the one the naive proxy left open, is volume. With no caps, that `10/10 served` is the shape of a denial-of-wallet attack: nothing between a caller and your model budget. gouncer caps the rate per client, so a burst past the limit is refused rather than forwarded:

```
== flood: a bounded burst of 10 requests ==
  naive proxy:  10/10 served, 0 refused (nothing says stop)
  gouncer:      3 served, 7 refused with 429 once past the rate cap
```

The first three go through, the burst allowance, and the rest come back `429 Too Many Requests`. The cap is deliberately tiny here so ten requests show it; the point is that there is a number, and past it the gateway says no instead of spending. The same holds for an oversized request, a cheap way to run up cost or memory:

```
== oversized body ==
  naive proxy:  HTTP 200 (a 4 KB body sailed through)
  gouncer:      HTTP 413 (request body too large) (cap is 512 bytes)
```

A four kilobyte body through the naive proxy, a `413 Request Entity Too Large` through the gateway. None of this is clever, and that is the point: the caps are small, boring numbers, and the reason to have them is that without them the limit is whatever an attacker, or a bug in your own loop, decides.

## Conclusion

So the agent now has its lens: a log that cannot be quietly forged, and a gateway that every model call passes through, where the host is fixed, the body is capped, and the rate is capped, all of it audited under one trace id. An unknown host is a 400, a burst is a 429, a fat request is a 413, and each refusal is a real status code you watched the gateway return, not a promise.

That finishes the observability part. We built the ability to see and to say no at the door before building a single model-facing defense, because now when we start closing the red findings from the baseline, we will be able to watch each attack happen and watch it stop. The next post ties the two halves together with the trace id that runs through everything, and the operator view it buys you for free. After that, Part 3 begins, and we start turning the red board green.
