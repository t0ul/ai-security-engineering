---
title: "A gateway you cannot route around"
description: "A model call should have exactly one way out, through a choke point you control. This post builds the naive proxy first, lets it be steered at an internal service and flooded with no limit, then puts gouncer in front: an allowlist of model names, a body cap, and a rate cap, so an unknown host is a 400, a burst is a 429, and an oversized body is a 413."
date: 2025-10-21
draft: false
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

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init gateway`, then `go get github.com/t0ul/gouncer@` followed by the version on the `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/03.2-a-gateway-you-cannot-route-around

go 1.27.1

// The real security gateway. No local overrides.
require github.com/t0ul/gouncer v0.0.0-20261006145250-6abb912f985c
```
<!-- /listing -->

## The spine of the program

`main` stands up the two servers the whole lesson turns on: a legitimate model backend, the only place a call should go, and an internal service the agent must never reach through a model call, which counts how often it is hit and answers with a secret. Then it runs the three attacks and prints the receipt only if all three came out the way this post says.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 03.2, a gateway you cannot route
// around.
//
// Run it with:
//
//	go run .
//
// Every model call should pass through one choke point. A naive proxy that lets
// the caller choose the upstream is a server-side request forgery waiting to
// happen, and with no caps it is a flood waiting to happen. This demo fires both
// attacks at a naive proxy and at the real gouncer gateway. Everything runs on
// throwaway local servers; the flood is a bounded burst of ten requests, never a
// real service.
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/t0ul/gouncer"
)

func main() {
	// A legitimate model backend (the only place a call should go) and an internal
	// service the agent must never reach through a model call.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"[]"}}]}`)
	}))
	defer backend.Close()

	internalHits := 0
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHits++
		io.WriteString(w, "INTERNAL-SECRET: db_password=hunter2")
	}))
	defer internal.Close()

	ssrf := ssrfBlocked(backend.URL, internal.URL, &internalHits)
	flood := floodBounded(backend.URL)
	body := bodyCapped(backend.URL)

	fmt.Println()
	if ssrf && flood && body {
		fmt.Println("attack blocked (the receipt)")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

## I. A proxy the caller can aim

Here is the model proxy I would write first, `naiveProxy`. It reads where to send the call from the request and forwards it there. Under it is the control, `gouncerGateway`, which the next section explains; read the naive one now.

<!-- listing: main.go#the two proxies -->
```go
// ---- the two proxies ----

// naiveProxy is the gateway I would write first: it reads the upstream to call
// from the request and forwards there, with no allowlist and no caps. Letting the
// caller pick the host is the SSRF; having no caps is the flood.
func naiveProxy() http.HandlerFunc {
	client := &http.Client{}
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstream := extractUpstream(body) // the caller's chosen host, trusted
		resp, err := client.Post(upstream+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

// gouncerGateway is the real control: an allowlist of logical model names, a body
// cap, and a per-client rate cap. The caller can never name a host, only a model,
// and an unknown model is refused.
func gouncerGateway(backendURL string) http.Handler {
	gw, err := gouncer.New(gouncer.Config{
		Routes:        map[string]gouncer.Route{"planner": {Upstream: backendURL}},
		MaxBodyBytes:  512,
		RatePerMinute: 60,
		RateBurst:     3,
	})
	if err != nil {
		panic(err)
	}
	return gw
}
```
<!-- /listing -->

It works, and that one line, trusting the caller's chosen host, is the hole. The first attack sends the proxy a request that names the internal service as the upstream, then sends the same request to the gateway:

<!-- listing: main.go#attack 1: SSRF -->
```go
// ---- attack 1: SSRF ----

func ssrfBlocked(backendURL, internalURL string, hits *int) bool {
	fmt.Println("== SSRF: a model call aimed at an internal service ==")

	// Undefended: the attacker names the internal service as the upstream.
	np := httptest.NewServer(naiveProxy())
	defer np.Close()
	*hits = 0
	req := fmt.Sprintf(`{"model":"planner","upstream":%q,"messages":[]}`, internalURL)
	code, _ := post(np.URL, "attacker", req)
	leaked := *hits > 0
	fmt.Printf("  naive proxy:  HTTP %d, internal service hit %d time(s)%s\n", code, *hits,
		tag(leaked, "  <- SSRF: reached the internal host and leaked its secret"))

	// Defended: gouncer takes no upstream from the caller; the attacker can only
	// name a model, and the internal service is not one.
	gw := httptest.NewServer(gouncerGateway(backendURL))
	defer gw.Close()
	*hits = 0
	code2, body2 := post(gw.URL, "attacker-ssrf", `{"model":"internal","upstream":"`+internalURL+`","messages":[]}`)
	fmt.Printf("  gouncer:      HTTP %d %s, internal service hit %d time(s)\n", code2, errMsg(body2), *hits)
	return leaked && code2 == http.StatusBadRequest && *hits == 0
}
```
<!-- /listing -->

The naive half of that:

```
== SSRF: a model call aimed at an internal service ==
  naive proxy:  HTTP 200, internal service hit 1 time(s)  <- SSRF: reached the internal host and leaked its secret
```

The proxy did exactly what it was told. It fetched the internal host and handed its secret back, 200, no complaint. On a real machine that internal host is the cloud metadata endpoint handing out credentials, or an admin service on localhost, and the thing steering the proxy is an instruction in an email the model read. A call that was supposed to reach a model reached inside the network instead.

## II. A door that only knows model names

The fix is to take the choice of host away from the caller entirely. I put `gouncer` in front, a small zero-dependency Go library that is a security gateway for the OpenAI-compatible API that local and hosted models speak. Its whole idea is in one sentence: a caller names a logical model, never a host, and the gateway maps allowed model names to their upstreams. There is no field a request can set to change where the call goes. An unknown model is refused, so server-side request forgery is not mitigated, it is removed, because the lever the attack needs does not exist. It also caps bodies, limits rate and concurrency per client, and carries one trace id end to end into the gledger log from the last post, so every call is one auditable record through one door.

Configuring it is the allowlist and the caps, nothing more; that is all `gouncerGateway`, above, does. `planner` is the one model this gateway will route, and it goes to the one upstream I named. Send the same attack, a request naming the internal service, and the model name it carries is not on the allowlist:

```
  gouncer:      HTTP 400 (model not allowed), internal service hit 0 time(s)
```

A 400, and the internal service was hit zero times. The attacker could not even express the request, because the gateway's vocabulary is model names and `internal` is not one of them. The host is simply not something a caller gets to say.

## III. The flood and the fat request

The other door, the one the naive proxy left open, is volume. The second attack is a burst of ten requests at each:

<!-- listing: main.go#attack 2: flood (bounded) -->
```go
// ---- attack 2: flood (bounded) ----

func floodBounded(backendURL string) bool {
	fmt.Println("\n== flood: a bounded burst of 10 requests ==")
	const burst = 10

	np := httptest.NewServer(naiveProxy())
	defer np.Close()
	served := 0
	for i := 0; i < burst; i++ {
		if code, _ := post(np.URL, "flood", fmt.Sprintf(`{"model":"planner","upstream":%q,"messages":[]}`, backendURL)); code == 200 {
			served++
		}
	}
	fmt.Printf("  naive proxy:  %d/%d served, 0 refused (nothing says stop)\n", served, burst)

	gw := httptest.NewServer(gouncerGateway(backendURL))
	defer gw.Close()
	ok, limited := 0, 0
	for i := 0; i < burst; i++ {
		switch code, _ := post(gw.URL, "flood", `{"model":"planner","messages":[]}`); code {
		case 200:
			ok++
		case http.StatusTooManyRequests:
			limited++
		}
	}
	fmt.Printf("  gouncer:      %d served, %d refused with 429 once past the rate cap\n", ok, limited)
	return served == burst && limited > 0
}
```
<!-- /listing -->

With no caps, that `10/10 served` is the shape of a denial-of-wallet attack: nothing between a caller and your model budget. gouncer caps the rate per client, so a burst past the limit is refused rather than forwarded:

```
== flood: a bounded burst of 10 requests ==
  naive proxy:  10/10 served, 0 refused (nothing says stop)
  gouncer:      3 served, 7 refused with 429 once past the rate cap
```

The first three go through, the burst allowance, and the rest come back `429 Too Many Requests`. The cap is deliberately tiny here so ten requests show it; the point is that there is a number, and past it the gateway says no instead of spending. The same holds for an oversized request, a cheap way to run up cost or memory. The third attack is a four kilobyte body at each:

<!-- listing: main.go#attack 3: oversized body -->
```go
// ---- attack 3: oversized body ----

func bodyCapped(backendURL string) bool {
	fmt.Println("\n== oversized body ==")
	big := `{"model":"planner","messages":[{"role":"user","content":"` + repeat("A", 4096) + `"}]}`

	np := httptest.NewServer(naiveProxy())
	defer np.Close()
	code, _ := post2(np.URL, "body", `{"model":"planner","upstream":"`+backendURL+`","messages":[{"role":"user","content":"`+repeat("A", 4096)+`"}]}`)
	fmt.Printf("  naive proxy:  HTTP %d (a 4 KB body sailed through)\n", code)

	gw := httptest.NewServer(gouncerGateway(backendURL))
	defer gw.Close()
	code2, body2 := post2(gw.URL, "body", big)
	fmt.Printf("  gouncer:      HTTP %d %s (cap is 512 bytes)\n", code2, errMsg(body2))
	return code == 200 && code2 == http.StatusRequestEntityTooLarge
}
```
<!-- /listing -->

```
== oversized body ==
  naive proxy:  HTTP 200 (a 4 KB body sailed through)
  gouncer:      HTTP 413 (request body too large) (cap is 512 bytes)
```

A four kilobyte body through the naive proxy, a `413 Request Entity Too Large` through the gateway. None of this is clever, and that is the point: the caps are small, boring numbers, and the reason to have them is that without them the limit is whatever an attacker, or a bug in your own loop, decides.

The rest of the file is helpers: one `post` that sends a request with a per-scenario API key so each attack gets its own rate bucket, the naive proxy's reading of the caller's chosen host, and three for printing.

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

func post(base, key, body string) (int, string) { return post2(base, key, body) }

func post2(base, key, body string) (int, string) {
	req, _ := http.NewRequest("POST", base+"/v1/chat/completions", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", key) // give each scenario its own rate bucket
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func extractUpstream(body []byte) string {
	s := string(body)
	i := bytes_Index(s, `"upstream":"`)
	if i < 0 {
		return ""
	}
	s = s[i+len(`"upstream":"`):]
	j := bytes_Index(s, `"`)
	if j < 0 {
		return ""
	}
	return s[:j]
}

func bytes_Index(s, sub string) int { return bytes.Index([]byte(s), []byte(sub)) }

func errMsg(body string) string {
	if i := bytes_Index(body, `"message":"`); i >= 0 {
		s := body[i+len(`"message":"`):]
		if j := bytes_Index(s, `"`); j >= 0 {
			return "(" + s[:j] + ")"
		}
	}
	return ""
}

func tag(cond bool, s string) string {
	if cond {
		return s
	}
	return ""
}

func repeat(s string, n int) string {
	out := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, s[0])
	}
	return string(out)
}
```
<!-- /listing -->

That is the whole program. The folder's tests run the same three attacks, each a real HTTP request at a real server, and `make board` reruns them after every later lesson. Put this next to `main.go` as `contract_test.go` and run `go test`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Deterministic: no model. Each attack is a real HTTP request at a real gateway.
// The naive proxy lets the caller pick the host (SSRF) and has no caps (flood);
// gouncer refuses an unknown model (400), rate-limits a burst (429), and rejects
// an oversized body (413).

func fixtures(t *testing.T) (backend string, internal string, hits *int) {
	h := 0
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"[]"}}]}`)
	}))
	t.Cleanup(b.Close)
	in := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h++
		io.WriteString(w, "INTERNAL-SECRET")
	}))
	t.Cleanup(in.Close)
	return b.URL, in.URL, &h
}

func TestSSRFReachesInternalUndefendedBlockedDefended(t *testing.T) {
	backend, internal, hits := fixtures(t)
	if !ssrfBlocked(backend, internal, hits) {
		t.Fatal("naive proxy must reach the internal service; gouncer must refuse an unknown model and never contact it")
	}
}

func TestFloodBounded(t *testing.T) {
	backend, _, _ := fixtures(t)
	if !floodBounded(backend) {
		t.Fatal("naive proxy serves the whole burst; gouncer must return 429 past the rate cap")
	}
}

func TestBodyCapped(t *testing.T) {
	backend, _, _ := fixtures(t)
	if !bodyCapped(backend) {
		t.Fatal("naive proxy accepts the oversized body; gouncer must return 413")
	}
}
```
<!-- /listing -->

## Conclusion

So the agent now has its lens: a log that cannot be quietly forged, and a gateway that every model call passes through, where the host is fixed, the body is capped, and the rate is capped, all of it audited under one trace id. An unknown host is a 400, a burst is a 429, a fat request is a 413, and each refusal is a real status code you watched the gateway return, not a promise.

That finishes the observability part. We built the ability to see and to say no at the door before building a single model-facing defense, because now when we start closing the red findings from the baseline, we will be able to watch each attack happen and watch it stop. The next post ties the two halves together with the trace id that runs through everything, and the operator view it buys you for free. After that, Part 3 begins, and we start turning the red board green.

## Take it to your own project

Put one door in front of every model and outbound call:

- Route all model and network calls through a single gateway; no component calls out directly.
- Allowlist the hosts it may reach and deny everything else by default.
- Cap rate, concurrency, and body size at that one place.
- Make it the only path, so a new feature cannot quietly open a second door.
