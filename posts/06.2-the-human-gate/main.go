// Command main is the runnable demo for lesson 06.2, the human gate.
//
// Run it with:
//
//	go run .
//
// Some actions are irreversible: putting an event on your real calendar, acting on
// a link. No scoped capability should grant those automatically; they need a human.
// But a one-click "yes" can be forged, clickjacked, or rubber-stamped. This demo
// runs a naive accept that acts on any approval, then the agent's real gate, which
// acts only on an approval that echoes a nonce shown with the evidence, so a blind
// or forged click cannot pass while a genuine human yes still can. Deterministic.
package main

import (
	"fmt"
	"os"

	"github.com/t0ul/emaildrop/pkg/hitl"
)

// calendar is the irreversible side the action touches. In the real agent this is
// your actual calendar; here it is a slice we can inspect.
var calendar []string

func addToCalendar(event string) { calendar = append(calendar, event) }

const event = "Principal's Gala (Dec 25)"

func main() {
	calendar = nil
	landed := naiveAcceptsAnything()

	calendar = nil
	held := gateHoldsButLetsRealThrough()

	fmt.Println()
	if landed && held {
		fmt.Println("attack blocked (the receipt): a bare yes acts on the naive path, refused by the gate")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- before: one-click accept ----

// naiveAccept is the approval I would write first: if the request says approve, do
// the thing. A single boolean is the entire check, which is what makes it forgeable.
func naiveAccept(approve bool) {
	if approve {
		addToCalendar(event)
	}
}

func naiveAcceptsAnything() bool {
	fmt.Println("== before: a one-click accept ==")
	// A forged request: approval claimed with nobody having seen anything.
	naiveAccept(true)
	landed := len(calendar) > 0
	fmt.Printf("  a bare \"approve: true\" put it on the calendar: %v (calendar: %v)\n", landed, calendar)
	return landed
}

// ---- after: evidence-first, nonce-bound approval ----

func gateHoldsButLetsRealThrough() bool {
	fmt.Println("\n== after: the evidence-first human gate ==")

	// The agent issues a challenge: a summary, the evidence to review, and a nonce
	// shown with them. Acting requires echoing THAT nonce back.
	req := hitl.NewRequest(
		"Add "+event+" to your calendar",
		"source: newsletter.txt",
		"this event's date is not written in the email body",
	)

	// A forged or blind approval: it says yes, but it cannot echo the nonce it
	// never saw.
	okForged, _ := req.Confirm("", true)
	actOn(okForged)
	fmt.Printf("  forged approval (no nonce):      confirmed=%v, calendar: %v\n", okForged, calendar)

	// A clickjacked guess at the nonce: still wrong.
	okGuess, _ := req.Confirm("not-the-real-nonce", true)
	actOn(okGuess)
	fmt.Printf("  guessed nonce:                   confirmed=%v, calendar: %v\n", okGuess, calendar)

	// A genuine human, who was shown the evidence and the nonce, echoes it back.
	okReal, _ := req.Confirm(req.Nonce(), true)
	actOn(okReal)
	fmt.Printf("  genuine approval (real nonce):   confirmed=%v, calendar: %v\n", okReal, calendar)

	// The gate held against the forgeries and let the real yes through.
	return !okForged && !okGuess && okReal && len(calendar) == 1
}

// actOn performs the irreversible action only when the approval was confirmed.
func actOn(confirmed bool) {
	if confirmed {
		addToCalendar(event)
	}
}
