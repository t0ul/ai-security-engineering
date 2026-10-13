// Command main is the runnable demo for lesson 14.6, output authenticity.
//
// Run it with:
//
//	go run .
//
// The agent produces artifacts people act on: a calendar file they import, a
// summary they read. If anyone can produce a file that claims to be from the agent,
// or alter one after the agent made it, then "the agent said so" means nothing. The
// agent signs what it produces with a content credential, so a consumer can verify
// the artifact came from the agent and was not changed. A forged or tampered one
// fails that check. Deterministic, no model.
package main

import (
	"fmt"
	"os"

	"github.com/t0ul/emaildrop/pkg/provenance"
)

// the agent's genuine output: a calendar event it extracted and emitted.
const artifact = "BEGIN:VEVENT\nSUMMARY:Multicultural Potluck\nDTSTART:20261023\nLOCATION:School Cafeteria\nEND:VEVENT"

// a tampered copy: the time was changed after the agent signed it.
const tampered = "BEGIN:VEVENT\nSUMMARY:Multicultural Potluck\nDTSTART:20261024\nLOCATION:Attacker Hall\nEND:VEVENT"

type candidate struct {
	name    string
	content string
	mark    provenance.Mark
}

func main() {
	agent, agentPub, _ := provenance.NewSigner("emaildrop-agent")
	attacker, _, _ := provenance.NewSigner("attacker")
	// A consumer that trusts only the agent's key.
	verifier := provenance.NewVerifier().Trust("emaildrop-agent", agentPub)

	realMark := agent.Sign([]byte(artifact))

	candidates := []candidate{
		{"genuine", artifact, realMark}, // the agent's own signed output
		{"forged", "BEGIN:VEVENT\nSUMMARY:Pay tuition now\nEND:VEVENT", attacker.Sign([]byte("BEGIN:VEVENT\nSUMMARY:Pay tuition now\nEND:VEVENT"))}, // attacker-signed fake
		{"tampered", tampered, realMark}, // the agent's mark over altered content
	}

	// ---- before: trust anything that claims to be from the agent ----
	fmt.Println("== before: a consumer that does not verify ==")
	accepted := 0
	for _, c := range candidates {
		if c.name != "genuine" { // the two attacks
			accepted++
		}
		fmt.Printf("  %-9s accepted as genuine: true\n", c.name)
	}
	fmt.Printf("  output-forgery ASR: %d%% (%d/2 forgeries accepted)\n", accepted*100/2, accepted)

	// ---- after: verify the content credential ----
	fmt.Println("\n== after: verify the content credential ==")
	landed := 0
	var genuineOK bool
	for _, c := range candidates {
		err := verifier.Verify([]byte(c.content), c.mark)
		ok := err == nil
		if c.name == "genuine" {
			genuineOK = ok
		} else if ok {
			landed++ // a forgery that still verified is a breach
		}
		fmt.Printf("  %-9s verifies: %-5v %s\n", c.name, ok, reason(c.name, err))
	}
	fmt.Printf("  output-forgery ASR: %d%% (%d/2)\n", landed*100/2, landed)

	fmt.Println()
	if accepted == 2 && landed == 0 && genuineOK {
		fmt.Println("attack blocked (the receipt): a consumer that does not verify accepts a forged and a tampered artifact; verifying the content credential rejects both while the agent's genuine output still verifies")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

func reason(name string, err error) string {
	if err == nil {
		return "(the agent produced this, unaltered)"
	}
	if name == "tampered" {
		return "(content changed after signing)"
	}
	return "(not signed by the agent)"
}
