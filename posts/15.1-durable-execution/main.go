// Command main is the runnable demo for lesson 15.1, durable execution and
// resume-verify.
//
// Run it with:
//
//	go run .
//
// A long-running agent crashes and resumes, replaying completed steps from a
// durable log instead of redoing them. That log is on disk, so it is attackable:
// edit a recorded step and the agent resumes into a state it never actually
// reached, an approval that was a denial, a command it never chose. The agent's
// log is a hash chain, so a tampered checkpoint does not verify, and resume fails
// closed rather than trusting a forged state. Deterministic, no model.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/t0ul/emaildrop/pkg/durable"
)

func main() {
	dir, _ := os.MkdirTemp("", "durable")
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "steps.log")

	runWorkflow(path)
	tamper(path) // an attacker flips the denial to an approval on disk

	fmt.Println("== before: resume by trusting the log on disk ==")
	decision := naiveResume(path)
	adopted := decision == "approved"
	fmt.Printf("  the resumed agent reads the approval decision as: %q\n", decision)
	fmt.Printf("  it would issue the refund the human denied: %v\n", adopted)

	fmt.Println("\n== after: resume-verify the hash chain ==")
	_, err := durable.Open(path) // Open loads AND verifies the chain
	refused := errors.Is(err, durable.ErrTampered)
	fmt.Printf("  reopening the tampered log: %v\n", err)
	fmt.Printf("  resume refused on a tampered checkpoint: %v\n", refused)

	// A clean log of the same workflow still resumes, and the real decision stands.
	clean := filepath.Join(dir, "clean.log")
	runWorkflow(clean)
	l, errClean := durable.Open(clean)
	real, _ := l.Lookup(session, "approve")
	fmt.Printf("  a clean log resumes (err=%v); the real decision is: %q\n", errClean, real.Output)

	fmt.Println()
	if adopted && refused && errClean == nil && real.Output == "denied" {
		fmt.Println("attack blocked (the receipt): a naive resume trusts a forged checkpoint and reverses a decision; the hash chain refuses a tampered log and fails closed, while a clean log resumes with the real decision intact")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- the workflow, the tamper, and what resume reads ----

const session = "refund-session"

// runWorkflow records three completed steps to a durable log on disk. The middle
// step is a security decision: the human DENIED the refund.
func runWorkflow(path string) {
	l, _ := durable.Open(path)
	_, _, _ = l.Do(session, "model", "plan", func() (string, error) { return "outline the refund request", nil })
	_, _, _ = l.Do(session, "tool", "approve", func() (string, error) { return "denied", nil })
	_, _, _ = l.Do(session, "model", "finalize", func() (string, error) { return "close the ticket", nil })
}

// tamper edits the persisted log the way an attacker with disk access would: it
// flips the recorded approval decision. The step's stored hash is left as-is, so
// the chain no longer verifies.
func tamper(path string) {
	b, _ := os.ReadFile(path)
	out := strings.Replace(string(b), `"output":"denied"`, `"output":"approved"`, 1)
	_ = os.WriteFile(path, []byte(out), 0o600)
}

// approveDecision reads what the "approve" step recorded, as a resuming agent would.
func approveDecision(steps []durable.Step) string {
	for _, s := range steps {
		if s.Key == "approve" {
			return s.Output
		}
	}
	return ""
}

// ---- before: resume by trusting the log on disk ----

// naiveResume reads the persisted steps and uses them without checking the chain.
func naiveResume(path string) string {
	b, _ := os.ReadFile(path)
	var steps []durable.Step
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		var s durable.Step
		if json.Unmarshal([]byte(line), &s) == nil {
			steps = append(steps, s)
		}
	}
	return approveDecision(steps)
}
