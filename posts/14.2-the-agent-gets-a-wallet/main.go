// Command main is the runnable demo for lesson 14.2, the agent gets a wallet.
//
// Run it with:
//
//	go run .
//
// The agent is a non-human identity that holds credentials to act: short-lived,
// signed, scoped grants it presents to send email, fetch a URL, write a calendar.
// A wallet of them. When one leaks, revocation has to mean something immediately,
// and the classic bug is that it does not: a bearer token that only gets its expiry
// checked keeps working after it is revoked, until it times out on its own. The
// agent's authority consults a revocation list on every check, and the kill switch
// revokes the whole wallet at once. Deterministic, no model.
package main

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/emaildrop/pkg/provenance"
)

func main() {
	undef := revokedStillWorksUndefended()
	def := revocationHoldsDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): an expiry-only check honors a revoked token until it times out; the agent's authority refuses a revoked grant the instant it is revoked, still honors a fresh one, and the kill switch revokes the whole wallet at once")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- the capability, and the authority ----

// want is the scope the agent's grant is checked against: permission to send email.
var want = controlplane.Capability{Action: controlplane.ActionExport, Resource: "email", Tenant: "public"}

func newAuthority() (*controlplane.Authority, func(string) controlplane.Grant) {
	signer, pub, _ := provenance.NewSigner("control-plane")
	authz := controlplane.NewAuthority(signer, provenance.NewVerifier().Trust("control-plane", pub))
	issue := func(subject string) controlplane.Grant {
		g, _ := authz.IssueScoped(controlplane.Capability{Subject: subject, Action: controlplane.ActionExport, Resource: "email", Tenant: "public"}, nil, time.Hour)
		return g
	}
	return authz, issue
}

// ---- before: a bearer token checked only for expiry ----

// naiveCheck is the mistake: a grant is honored as long as it has not expired.
// Nothing consults whether it was revoked, so revocation does nothing until the
// token times out on its own, an hour, a day, however long it was minted for.
func naiveCheck(g controlplane.Grant) bool {
	return time.Now().UTC().Before(g.Expires)
}

func revokedStillWorksUndefended() bool {
	fmt.Println("== before: a token checked only for expiry ==")
	authz, issue := newAuthority()
	g := issue("agent")
	authz.Revoke("agent") // the credential leaked; we revoke it right now
	stillWorks := naiveCheck(g)
	fmt.Printf("  after revocation, the naive check still honors the token: %v\n", stillWorks)
	return stillWorks
}

// ---- after: revocation consulted on every verify ----

func revocationHoldsDefended() bool {
	fmt.Println("\n== after: the authority consults revocation ==")
	authz, issue := newAuthority()

	g := issue("agent")
	authz.Revoke("agent")
	_, errRevoked := authz.Verify(g, want)
	fmt.Printf("  revoked token verifies: %v\n", errRevoked)

	// Revocation is targeted, not a blanket denial: a fresh grant still works.
	fresh := issue("agent-2")
	_, errFresh := authz.Verify(fresh, want)
	fmt.Printf("  a fresh, un-revoked grant still works:  %v\n", errFresh == nil)

	// The kill switch revokes the whole wallet at once: even a valid grant is
	// refused while the agent is halted.
	killed := false
	authz.Halted = func() bool { return killed }
	good := issue("agent-3")
	_, before := authz.Verify(good, want)
	killed = true
	_, after := authz.Verify(good, want)
	fmt.Printf("  kill switch: same grant before=%v, after engage=%v\n", before, after)

	return errors.Is(errRevoked, controlplane.ErrGrantRevoked) &&
		errFresh == nil &&
		before == nil && errors.Is(after, controlplane.ErrGrantRevoked)
}
