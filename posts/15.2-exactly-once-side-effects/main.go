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

const session = "orders"

// paid records every refund that actually left: a duplicate here is a double charge.
var paid []string

func reset() { paid = nil }

// issueRefund is the real side effect: money moves.
func issueRefund(order string) (string, error) {
	paid = append(paid, order)
	return "refunded " + order, nil
}

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
