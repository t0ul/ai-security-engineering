// Command main is the runnable demo for lesson 14.4, incident response and the
// kill switch.
//
// Run it with:
//
//	go run .
//
// The kill switch is the control you reach for in an incident, so it has to be two
// things an ad-hoc flag is not: governed, because an attacker who can flip the
// switch can flip it OFF mid-incident or ON as a denial of service, and layered,
// because a single full-stop is its own outage, a deny storm against every
// legitimate user. The agent's safety gate requires a break-glass capability to
// toggle, and stops side effects without stopping reads. Deterministic, no model.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/emaildrop/pkg/provenance"
)

func main() {
	undef := ungovernedUndefended()
	def := governedLayeredDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): an ungoverned switch anyone can flip and only a full halt is its own incident; the governed switch refuses an unauthorized toggle, and its layered levels stop side effects while reads keep serving, escalating to a full halt only when warranted")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- the authority, and the safety scope ----

func newAuthority() (*controlplane.Authority, controlplane.Grant, controlplane.Grant) {
	signer, pub, _ := provenance.NewSigner("control-plane")
	authz := controlplane.NewAuthority(signer, provenance.NewVerifier().Trust("control-plane", pub))
	op, _ := authz.IssueScoped(controlplane.Capability{Subject: "sre", Action: controlplane.ActionWrite, Resource: "safety", Tenant: "public"}, nil, time.Hour)
	app, _ := authz.IssueScoped(controlplane.Capability{Subject: "app", Action: controlplane.ActionList, Resource: "corpus", Tenant: "public"}, nil, time.Hour)
	return authz, op, app
}

// safetyScope is the capability toggling the kill switch requires.
var safetyScope = controlplane.Capability{Action: controlplane.ActionWrite, Resource: "safety", Tenant: "public"}

// ---- before: ungoverned, all-or-nothing ----

func ungovernedUndefended() bool {
	fmt.Println("== before: an ungoverned, all-or-nothing kill switch ==")

	// Anyone can toggle it: an attacker disengages it in the middle of an incident.
	s := controlplane.NewSafety(nil)
	s.Set("sre", controlplane.LevelHalt)      // responder halts the agent
	s.Set("attacker", controlplane.LevelNone) // ungoverned: attacker flips it back off
	flippedOff := s.AllowToolExec()
	fmt.Printf("  attacker flipped the switch back off mid-incident: %v\n", flippedOff)

	// The only stop is a full halt, which refuses even a safe read: a self-inflicted outage.
	h := controlplane.NewSafety(nil)
	h.Set("sre", controlplane.LevelHalt)
	readRefused := !h.AllowRequest()
	fmt.Printf("  the only stop is a full halt; a safe read is refused too: %v (deny storm)\n", readRefused)

	return flippedOff && readRefused
}

// ---- after: governed toggle, layered levels ----

// toggleGoverned changes the kill level only for a break-glass capability on the
// safety resource. VerifyGlass is the emergency path: it ignores revocation and
// halt (so you can always recover) but still demands the right scope.
func toggleGoverned(authz *controlplane.Authority, g controlplane.Grant, s *controlplane.Safety, by string, level controlplane.KillLevel) error {
	if _, err := authz.VerifyGlass(g, safetyScope); err != nil {
		return err
	}
	s.Set(by, level)
	return nil
}

func governedLayeredDefended() bool {
	fmt.Println("\n== after: governed toggle, layered levels ==")
	authz, op, app := newAuthority()
	s := controlplane.NewSafety(nil)

	// The app role cannot toggle the switch: wrong scope.
	errApp := toggleGoverned(authz, app, s, "app", controlplane.LevelNone)
	fmt.Printf("  app role tries to toggle: refused: %v\n", errApp)

	// The responder engages the layered level: stop side effects, keep reads.
	errOp := toggleGoverned(authz, op, s, "sre", controlplane.LevelBlockTools)
	toolBlocked := !s.AllowToolExec()
	readAllowed := s.AllowRequest()
	fmt.Printf("  responder engages block-tools (authorized): applied=%v\n", errOp == nil)
	fmt.Printf("    a side-effecting tool: %s\n", yn(toolBlocked, "blocked", "RUNS"))
	fmt.Printf("    a safe read:           %s   (contained, no storm)\n", yn(readAllowed, "allowed", "REFUSED"))

	// And the responder can still escalate to a full halt when it is warranted.
	_ = toggleGoverned(authz, op, s, "sre", controlplane.LevelHalt)
	fullStop := !s.AllowRequest() && !s.AllowToolExec()
	fmt.Printf("  responder escalates to full halt when warranted: everything stopped=%v\n", fullStop)

	return errApp != nil && errOp == nil && toolBlocked && readAllowed && fullStop
}

// ---- helpers ----

func yn(b bool, yes, no string) string {
	if b {
		return yes
	}
	return no
}
