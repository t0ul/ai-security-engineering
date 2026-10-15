---
title: "Exactly-once side effects"
description: "A calendar write done twice is annoying. A refund sent twice is money gone, and agents retry constantly: a timeout, a resume after a crash, a client that never saw the response. The naive answer fires the side effect on every attempt. The agent routes dangerous actions through a durable ledger keyed by an idempotency key, which fires the effect exactly once on success and still lets a genuinely failed attempt be retried until it completes."
date: 2026-09-08
draft: false
tags: ["ai-security", "golang", "reliability", "idempotency"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## When a retry is dangerous

The calendar chapter met this once already: re-ingest an email and the same event lands twice. There the cost was a duplicate on a screen. This post is the version where the cost is real, because the agents that matter do not only write calendars, they take actions, send a message, issue a refund, move money, and they retry those actions all the time. A request times out and the caller tries again. A process crashes mid-task and resumes. A client never saw the response and resends. Retrying is correct behavior and usually free, right up until the thing being retried has a side effect, at which point once-per-attempt is a bug with a dollar figure.

The feature is the same durable ledger from earlier, pointed at its harder job. A control lesson, deterministic, no model:

```bash
cd posts/15.2-exactly-once-side-effects
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init exactlyonce`, then `go get github.com/t0ul/emaildrop@v0.0.0-20261011232453-771805568f76`, and put the code in one file called `main.go`.

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/15.2-exactly-once-side-effects

go 1.27.1

// The agent's real durable exactly-once ledger. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76
```
<!-- /listing -->

## The spine of the program

`main` runs the naive retry and then the ledger, and the receipt needs the first to pay twice and the second to pay once on a retried success and once on a retried failure.

<!-- listing: main.go#head -->
```go
// Command main is the runnable demo for lesson 15.2, exactly-once side effects.
//
// Run it with:
//
//	go run .
//
// A calendar write done twice is annoying. A refund sent twice is money gone, and
// agents retry: a timeout, a resume after a crash, a client that did not see the
// response. The naive answer, "do the action", fires the side effect on every
// attempt. The agent routes dangerous actions through a durable ledger keyed by an
// idempotency key, which fires the effect exactly once on success, and still lets a
// genuinely FAILED attempt be retried until it completes. Deterministic, no model.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/t0ul/emaildrop/pkg/durable"
)

func main() {
	undef := retryDoublesUndefended()
	def := exactlyOnceDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("the receipt: a naive retry double-charges; the durable ledger fires the refund exactly once on success, replays a retry of a completed action, and still lets a genuinely failed attempt be retried to completion")
	} else {
		fmt.Println("the side effect did not fire exactly once")
		os.Exit(1)
	}
}
```
<!-- /listing -->

The side effect is a slice of refunds that left; a duplicate there is a double charge.

<!-- listing: main.go#the orders, and the refund that pays -->
```go
// ---- the orders, and the refund that pays ----

const session = "orders"

// paid records every refund that actually left: a duplicate here is a double charge.
var paid []string

func reset() { paid = nil }

// issueRefund is the real side effect: money moves.
func issueRefund(order string) (string, error) {
	paid = append(paid, order)
	return "refunded " + order, nil
}
```
<!-- /listing -->

## Retry the refund, charge it twice

Here is the naive action handler doing the obvious thing: a refund is requested, so issue the refund. Then the caller times out and retries the same refund.

<!-- listing: main.go#before: the action fires on every attempt -->
```go
// ---- before: the action fires on every attempt ----

func retryDoublesUndefended() bool {
	fmt.Println("== before: a retried action re-fires the refund ==")
	reset()
	// A timeout: the caller did not see the first response and retries the same refund.
	_, _ = issueRefund("order-42")
	_, _ = issueRefund("order-42")
	fmt.Printf("  refund issued per attempt; order-42 paid %d times\n", len(paid))
	return len(paid) == 2
}
```
<!-- /listing -->

```
== before: a retried action re-fires the refund ==
  refund issued per attempt; order-42 paid 2 times
```

Two refunds for one order. The second attempt was not a mistake by the caller; retrying a request you did not get a response to is exactly what a correct client does. The mistake is that the action fired once per attempt instead of once per intent, and nothing connected the second request to the first. For a refund that is a double payout; for an outbound email it is a double send; for anything that touches the world it is the same shape of error.

## Once per intent, keyed

The fix is an idempotency key: a stable name for this action, `refund:order-42`, that is the same across every retry of it. The agent runs the action through the durable ledger under that key, so the ledger can tell a fresh action from a repeat of one it already completed. The same function then runs a refund whose first attempt fails, which the section after this one reads.

<!-- listing: main.go#after: durable, keyed by an idempotency key -->
```go
// ---- after: durable, keyed by an idempotency key ----

func exactlyOnceDefended() bool {
	fmt.Println("\n== after: a durable ledger, keyed by the idempotency key ==")

	// A retried refund: the second attempt replays the first result, no second charge.
	reset()
	l, _ := durable.Open("")
	key := "refund:order-42"
	_, r1, _ := l.Do(session, "tool", key, func() (string, error) { return issueRefund("order-42") })
	_, r2, _ := l.Do(session, "tool", key, func() (string, error) { return issueRefund("order-42") })
	once := len(paid) == 1
	fmt.Printf("  retried refund: order-42 paid %d time (first replayed=%v, second replayed=%v)\n", len(paid), r1, r2)

	// A transiently FAILED attempt is not recorded, so a retry actually re-runs it
	// and completes: at-least-once on failure, exactly-once on success.
	reset()
	l2, _ := durable.Open("")
	attempts := 0
	flaky := func() (string, error) {
		attempts++
		if attempts == 1 {
			return "", errors.New("payment provider timeout") // no money moved
		}
		return issueRefund("order-99")
	}
	_, _, err1 := l2.Do(session, "tool", "refund:order-99", flaky) // fails, not recorded
	_, _, err2 := l2.Do(session, "tool", "refund:order-99", flaky) // retried, completes
	completed := len(paid) == 1
	fmt.Printf("  flaky refund: attempt1 err=%v, retry completes; order-99 paid %d time\n", err1 != nil, len(paid))

	return once && !r1 && r2 && completed && err1 != nil && err2 == nil
}
```
<!-- /listing -->

```
== after: a durable ledger, keyed by the idempotency key ==
  retried refund: order-42 paid 1 time (first replayed=false, second replayed=true)
```

One refund. The first call ran the effect and recorded it under the key; the second call found the key already done and returned the recorded result without firing anything. The side effect happened once per intent, and the retry got the same answer the first attempt would have, which is exactly what the caller wanted when they retried. Idempotency is not blocking the retry, it is making the retry safe.

## Exactly once on success, not on failure

There is a subtlety that separates a real exactly-once ledger from a naive dedupe, and it is the part that matters when the crash lands in the worst place. The ledger records an action only when it succeeds. A failed attempt, a payment the provider rejected, a send that errored, is not recorded, because the effect did not happen, so a retry is allowed to actually run it.

```
  flaky refund: attempt1 err=true, retry completes; order-99 paid 1 time
```

The first attempt failed and moved no money, so it left no record; the retry ran the action for real and completed it, exactly once. If the ledger had cached the failure, the refund would have been lost forever; if it had cached a pretend success, the customer would never be paid. Recording only success gives you the right guarantee on both sides: an action that happened cannot happen again, and an action that did not happen is still free to. At-least-once delivery on failure, exactly-once effect on success, which is the honest promise a durable system can actually keep.

That is the whole program. Its tests assert both halves and that a failed attempt is never recorded; put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"errors"
	"testing"

	"github.com/t0ul/emaildrop/pkg/durable"
)

// Deterministic, no model. A naive retry double-charges; the durable ledger fires
// the refund exactly once on success, replays a retry of a completed action, and
// still lets a genuinely failed attempt be retried to completion.

func TestRetryDoublesUndefended(t *testing.T) {
	if !retryDoublesUndefended() {
		t.Fatal("a naive retry must double the side effect; otherwise there is nothing to make exactly-once")
	}
}

func TestDurableExactlyOnceAndRetryable(t *testing.T) {
	if !exactlyOnceDefended() {
		t.Fatal("the durable ledger must fire once on a retried success and complete a retried failure")
	}
}

func TestFailureIsNotRecorded(t *testing.T) {
	reset()
	l, _ := durable.Open("")
	calls := 0
	fn := func() (string, error) {
		calls++
		if calls == 1 {
			return "", errors.New("transient")
		}
		return issueRefund("o1")
	}
	// A failed attempt must NOT be cached (so it is retryable).
	if _, _, err := l.Do(session, "tool", "k", fn); err == nil {
		t.Fatal("a failing fn must return its error, not be recorded")
	}
	if _, replayed, _ := l.Do(session, "tool", "k", fn); replayed {
		t.Fatal("a previously-failed key must re-run fn, not replay")
	}
	if len(paid) != 1 {
		t.Fatalf("the retry must complete exactly once, paid=%d", len(paid))
	}
}
```
<!-- /listing -->

The next reliability post is about the agent's memory filling up: compaction, where a long conversation is summarized to fit the window, and the summary, being model-written over untrusted history, becomes one more place an instruction can hide.

## Take it to your own project

Survive a crash without firing twice:

- Make side effects idempotent, with an idempotency key.
- Journal the intent before you act.
- On resume, check whether the effect already fired before firing it again.
