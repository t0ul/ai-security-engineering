---
title: "Egress-deny and the one way out"
description: "The chamber has no network card, which is safe but too absolute: sometimes a tool legitimately needs the network. So the chamber gets exactly one way out, a broker on the host it has to ask, which dials only what the egress policy allows and tunnels raw bytes without ever reading the untrusted response. A naive broker dials anything; the real one dials only the allowlist."
date: 2026-01-27
draft: false
tags: ["ai-security", "golang", "agents", "isolation"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## Safe is not the same as useful

[The last post](/blog/the-detonation-chamber/) put untrusted code in a box with no network card, so a command from an email could run and reach nothing. That is safe, and it is also too absolute to be the final answer, because every so often a tool genuinely needs the network. The user asks what the weather is for an outdoor event, a tool needs to call a real API, and a chamber that can never reach anything cannot do that job. A box with no exits is secure the way a brick is secure.

So the chamber gets exactly one way out, and the shape of it is the whole lesson. It is not a network card the guest controls, because then the guest could reach anywhere. It is a broker that runs on the host, and the only thing the guest can do is ask it. When code inside the chamber wants the network, it opens the chamber's one channel to the broker and sends a single line, the host and port it wants to reach, and the broker decides. The guest never dials anything. It can only make a request that the host is free to refuse. This is a control lesson, deterministic, no model:

```bash
cd posts/07.3-egress-deny-and-the-broker
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init broker`, then `go get github.com/t0ul/emaildrop@` followed by the version on the `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/07.3-egress-deny-and-the-broker

go 1.27.1

// The agent's real egress broker and egress policy. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
```
<!-- /listing -->

## The spine of the program

`main` stands up the two hosts the lesson turns on: a legitimate one the tool is allowed to use, answering with the weather, and an internal service it must never reach through the broker, which counts how often it is hit. Both get a name, because the broker vets names, and the fixed resolver under `main` maps those names to the local test servers so the demo is deterministic.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 07.3, egress-deny and the one way
// out.
//
// Run it with:
//
//	go run .
//
// The detonation chamber has no network card, so code inside it cannot reach the
// network on its own. When a tool legitimately needs to, it asks a broker on the
// host to open the connection. This demo asks a naive broker that dials whatever it
// is told, and reaches an internal service it should not, then the agent's real
// broker, which gates every target through the egress policy and refuses the
// internal one while still allowing a legitimate host. Deterministic, no model.
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/t0ul/emaildrop/pkg/broker"
	"github.com/t0ul/emaildrop/pkg/netpolicy"
)

var internalHits int

func main() {
	// A legitimate host the tool is allowed to use, and an internal service it must
	// never reach through the broker.
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "ALLOWED: this week's weather")
	}))
	defer ok.Close()
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHits++
		io.WriteString(w, "INTERNAL-SECRET: db credentials")
	}))
	defer internal.Close()

	okTarget := "weather.example:" + port(ok.URL)
	internalTarget := "internal.example:" + port(internal.URL)

	landed := naiveReachesInternal(internalTarget)
	blocked := brokerGates(okTarget, internalTarget)

	fmt.Println()
	if landed && blocked {
		fmt.Println("attack blocked (the receipt): the naive broker dials anything; the real one dials only the allowlist")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}
```
<!-- /listing -->

<!-- listing: main.go#where a host resolves -->
```go
// ---- where a host resolves ----

// resolve is a fixed resolver: the demo's named hosts map to the loopback test
// servers; everything else resolves normally.
func resolve(host string) ([]net.IP, error) {
	switch host {
	case "weather.example", "internal.example":
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	return net.LookupIP(host)
}
```
<!-- /listing -->

## I. One door, with no lock

Here is the broker written the easy way: read the target the guest asked for, dial it, and tunnel the bytes. It is the chamber's one door, propped wide open. Under it, a tool inside the chamber, subverted by an email, asks it not for the weather API but for an internal service on the host's own network:

<!-- listing: main.go#before: a broker that dials whatever it is asked -->
```go
// ---- before: a broker that dials whatever it is asked ----

// naiveBroker is the egress helper written without a policy: read the target, dial
// it, tunnel. It is the chamber's one door, propped wide open.
func naiveBroker(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			line := make([]byte, 256)
			n, _ := conn.Read(line)
			target := strings.TrimSpace(string(line[:n]))
			// No policy, but it still has to resolve the name to dial it, the same
			// way the real broker does; the difference is it asks no questions.
			host, prt, _ := net.SplitHostPort(target)
			ips, _ := resolve(host)
			up, err := net.Dial("tcp", net.JoinHostPort(ips[0].String(), prt)) // no check at all
			if err != nil {
				fmt.Fprintf(conn, "ERR %v\n", err)
				return
			}
			defer up.Close()
			io.WriteString(conn, "OK\n")
			go io.Copy(up, conn)
			io.Copy(conn, up)
		}()
	}
}

func naiveReachesInternal(internalTarget string) bool {
	fmt.Println("== before: a broker with no policy ==")
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go naiveBroker(ln)

	internalHits = 0
	body, err := guestFetch(ln.Addr().String(), "http://"+internalTarget+"/")
	fmt.Printf("  the guest asked the broker for %s\n  it got back: %q (internal service hit %d time(s))\n",
		internalTarget, body, internalHits)
	_ = err
	return internalHits > 0
}
```
<!-- /listing -->

What comes back:

```
== before: a broker with no policy ==
  the guest asked the broker for internal.example:<port>
  it got back: "INTERNAL-SECRET: db credentials" (internal service hit 1 time(s))
```

The isolation from the last post just evaporated. The chamber has no network card, exactly as designed, and it did not need one, because the broker reached the internal service on its behalf and handed the secret back through the tunnel. A door with no lock is not better than no wall; it is a wall with a hole cut in a convenient shape. The whole point of denying the chamber a NIC was to decide egress ourselves, and a broker that dials whatever it is told hands that decision right back to the attacker.

## II. A door that vets every target

![Egress is default-deny; the one audited broker is the only way out and vets every target against an allowlist.](egress-broker.svg)

*An allowlisted host is let through and logged; an attacker host or an internal address is refused. Nothing reaches the network except through the one vetted door.*

The real broker makes the decision it was built to make. The chamber still has no route of its own; when code inside needs the network it sends the broker a single line with the target, and before the broker dials anything it runs that target through the same egress authority from [the web-search post](/blog/a-search-tool-hits-a-local-server/), `netpolicy`: a default-deny allowlist, host resolution, a refusal of any address that is loopback, private, or the cloud-metadata endpoint, and dial-pinning to the vetted address so the name cannot rebind to somewhere else between the check and the connection. If the target passes, the broker answers `OK` and tunnels raw bytes. If it does not, the broker answers `ERR` and closes, never having opened a connection.

One detail in that design matters more than it looks. The broker tunnels bytes; it does not speak HTTP. The guest runs the TLS and the HTTP over the tunnel itself, which means the untrusted response from the network is parsed inside the chamber and never by the host. Execution stays in the box, content handling stays in the box, and the host's only job is to be a single, audited chokepoint that decides yes or no on a destination. Here is the same chamber, now behind the real broker, asking for the forbidden host and then a permitted one. The broker is the agent's own, with the policy from the web-search post as its one setting, and `guestFetch` is the in-chamber tool: it reaches the network only by asking the broker, through the broker's own HTTP client.

<!-- listing: main.go#after: the real broker gates every target -->
```go
// ---- after: the real broker gates every target ----

func brokerGates(okTarget, internalTarget string) bool {
	fmt.Println("\n== after: the agent's real broker ==")
	b := &broker.Broker{
		Policy:  netpolicy.Policy{Allow: []string{"weather.example"}, AllowLoopback: true, Resolve: resolve},
		Timeout: 5 * time.Second,
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go b.Serve(ln)

	// The forbidden internal host: refused before any connection is made.
	internalHits = 0
	_, errInt := guestFetch(ln.Addr().String(), "http://"+internalTarget+"/")
	fmt.Printf("  internal.example: %s (internal service hit %d time(s))\n", refusal(errInt), internalHits)

	// The allowlisted host: tunneled through, and the tool gets its data.
	okBody, errOk := guestFetch(ln.Addr().String(), "http://"+okTarget+"/")
	fmt.Printf("  weather.example:  %s\n", result(okBody, errOk))

	return internalHits == 0 && errInt != nil && errOk == nil && strings.Contains(okBody, "ALLOWED")
}

// guestFetch is the in-chamber tool: it reaches the network ONLY by asking the
// broker, using the broker's own HTTP client.
func guestFetch(brokerAddr, url string) (string, error) {
	dial := func(ctx context.Context) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", brokerAddr)
	}
	client := broker.HTTPClient(dial, 5*time.Second)
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}
```
<!-- /listing -->

The run:

```
== after: the agent's real broker ==
  internal.example: refused (internal service hit 0 time(s))
  weather.example:  tunneled, got: ALLOWED: this week's weather
```

The internal service is refused and never touched, zero hits, because its host is not on the allowlist. And the weather host, which is, tunnels through and the tool gets its answer. The chamber has exactly one way to reach the network, that way is the broker, and the broker reaches only what you allowed.

The three helpers at the bottom read a port off a test server and print a refusal or a result:

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

func port(u string) string { return u[strings.LastIndexByte(u, ':')+1:] }

func refusal(err error) string {
	if err == nil {
		return "REACHED (should not happen)"
	}
	return "refused"
}

func result(body string, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	return "tunneled, got: " + body
}
```
<!-- /listing -->

That is the whole program. Its tests stand up the same two servers in a small helper file and run both halves. Put these next to `main.go` as `helpers_test.go` and `contract_test.go`:

<!-- listing: helpers_test.go#all -->
```go
package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type servers struct{ ok, internal string }

func startServers(t *testing.T) servers {
	okS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "ALLOWED: this week's weather")
	}))
	t.Cleanup(okS.Close)
	inS := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHits++
		io.WriteString(w, "INTERNAL-SECRET")
	}))
	t.Cleanup(inS.Close)
	return servers{ok: "weather.example:" + port(okS.URL), internal: "internal.example:" + port(inS.URL)}
}
```
<!-- /listing -->

<!-- listing: contract_test.go#all -->
```go
package main

import "testing"

// Deterministic, no model. A naive broker tunnels to an internal service; the real
// broker refuses a target that is not on the egress allowlist and reaches only one
// that is.

func TestNaiveBrokerReachesInternal(t *testing.T) {
	ok := startServers(t)
	if !naiveReachesInternal(ok.internal) {
		t.Fatal("the naive broker should dial the internal service; otherwise there is nothing to fix")
	}
}

func TestBrokerRefusesInternalAllowsAllowlisted(t *testing.T) {
	s := startServers(t)
	if !brokerGates(s.ok, s.internal) {
		t.Fatal("the broker must refuse the internal target and tunnel the allowlisted one")
	}
}
```
<!-- /listing -->

## The shape of the exit

It is worth stating plainly what makes this safe, because it is the inverse of how egress usually works. Normally a process has a network connection and you try to restrict it. Here the chamber has no connection at all, and the only thing it can do is ask a separate program, on the other side of the isolation boundary, to make a specific connection for it. The decision lives entirely on the host, in one place, logged, default-deny. The attacker inside the chamber cannot open a socket, cannot resolve a name, cannot rebind a host, cannot read a response on the host's behalf. It can only ask, and the answer is no unless the destination was one you chose in advance. That is the difference between restricting egress and owning it.

## Conclusion

The chamber now has a single controlled, audited exit. Code inside can do its job when the job honestly needs the network, and it still cannot reach anything you did not permit, because the permission is decided outside the box by a broker that dials only the allowlist and tunnels bytes it never reads.

There is one hole left in the isolation story, and it is back at the very start of this chapter, in how a command gets into the chamber in the first place. We have been handing the chamber commands, and a command is a string, and a string handed to a shell is a small programming language an attacker can write in. The last post of this part closes that: arguments as an array checked against an allowlist, with no shell to be clever with, so that even the command going in is not a sentence the attacker gets to compose.

## Take it to your own project

Make the network default-deny with one audited exit:

- Deny all egress by default.
- Route the rare legitimate call through a single broker that vets the target.
- Allowlist hosts; log every call that goes out.
- No component gets its own quiet path to the network.
