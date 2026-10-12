// Command main is the runnable demo for lesson 03.1, a log you can trust.
//
// Run it with:
//
//	go run .
//
// An audit log is the evidence. If an attacker who reaches the disk can edit it
// and nothing notices, the evidence is theirs. This demo records the same run to a
// naive plain log and to gledger's hash-chained log, edits one line in each to
// rewrite what happened, and asks each log whether the edit is detectable. The
// plain log accepts the forgery; gledger's chain catches it. It also shows gledger
// redacting a secret that the plain log writes in the clear.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/t0ul/gledger"
)

func main() {
	dir, err := os.MkdirTemp("", "lesson-03.1-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	landed := tamperGoesUnnoticed(dir)
	blocked := tamperIsCaught(dir)
	redacted := secretIsRedacted(dir)

	fmt.Println()
	if landed && blocked && redacted {
		fmt.Println("attack blocked (the receipt)")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- the run both logs record ----
//
// Three steps of one traced request: a request starts, the injection guard BLOCKS
// an injected event, and a real event is written. The guard line is the one that
// matters: it is the record of the agent stopping an attack, so it is exactly what
// an intruder would rewrite.

type step struct{ trace, span, event, detail string }

func theRun() []step {
	return []step{
		{"4a4778fc", "request", "start", "source=newsletter.txt"},
		{"4a4778fc", "injection_guard", "blocked", "dropped injected event: Principal's Gala"},
		{"4a4778fc", "event_extractor", "written", "PTA Meeting 2026-09-24"},
	}
}

// ---- before: a plain log, and a tamper it cannot see ----

// plainLog is the audit log I would write first: one JSON-ish line per step,
// appended to a file. It is readable, it is honest about what happened, and it has
// no integrity at all, which is the whole problem.
func plainLog(path string, steps []step) error {
	var b strings.Builder
	for _, s := range steps {
		fmt.Fprintf(&b, `{"trace":%q,"span":%q,"event":%q,"detail":%q}`+"\n", s.trace, s.span, s.event, s.detail)
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// tamperGoesUnnoticed writes the plain log, rewrites the guard line from "blocked"
// to "allowed", and shows that re-reading the log simply believes the new version.
// A plain log has no way to know it was edited, so the attack lands.
func tamperGoesUnnoticed(dir string) bool {
	fmt.Println("== before: a plain log ==")
	path := filepath.Join(dir, "plain.log")
	if err := plainLog(path, theRun()); err != nil {
		panic(err)
	}
	// The intruder edits the evidence: the agent blocked the injected event; the
	// log will now say it allowed it.
	forge(path, `"event":"blocked"`, `"event":"allowed"`)
	data, _ := os.ReadFile(path)
	line := lineContaining(string(data), "injection_guard")
	fmt.Printf("  after the edit, the log reads:\n    %s\n", line)
	fmt.Println("  re-reading a plain log just believes the file. the forgery is invisible.")
	return strings.Contains(line, `"allowed"`) // the tamper stands, undetected
}

// ---- after: the same tamper, against gledger ----

// tamperIsCaught writes the same run to a gledger log, makes the exact same edit,
// then runs gledger's own verifier over the file on disk.
func tamperIsCaught(dir string) bool {
	fmt.Println("\n== after: gledger's hash-chained log ==")
	path := filepath.Join(dir, "audit.jsonl")
	log, err := gledger.Open(path, "agent")
	if err != nil {
		panic(err)
	}
	for _, s := range theRun() {
		log.Emit(s.trace, s.span, s.event, gledger.F{"detail": s.detail})
	}

	ok, n := gledger.VerifyFile(path)
	fmt.Printf("  before the edit: chain_ok=%v over %d records\n", ok, n)

	// The same forgery: flip the guard record from blocked to allowed.
	forge(path, `"event":"blocked"`, `"event":"allowed"`)

	ok, n = gledger.VerifyFile(path)
	fmt.Printf("  after the edit:  chain_ok=%v (the chain breaks at the tampered record)\n", ok)
	return !ok // the tamper is detected
}

// ---- a secret in the log ----

// secretIsRedacted shows the other half of "a log you can trust": a value that
// should never be written at all. The agent logs a config line that contains a
// token. The plain log writes it verbatim; gledger redacts it on the way in.
func secretIsRedacted(dir string) bool {
	fmt.Println("\n== also: a secret in the record ==")
	const secret = "token=ghp_s3cr3tLiveKey99"

	plain := filepath.Join(dir, "plain2.log")
	_ = plainLog(plain, []step{{"9f12", "config", "loaded", secret}})
	pdata, _ := os.ReadFile(plain)
	pline := lineContaining(string(pdata), "config")
	fmt.Printf("  plain log:   %s\n", pline)

	gl := filepath.Join(dir, "audit2.jsonl")
	log, _ := gledger.Open(gl, "agent")
	log.Emit("9f12", "config", "loaded", gledger.F{"detail": secret})
	gdata, _ := os.ReadFile(gl)
	gline := lineContaining(string(gdata), "config")
	fmt.Printf("  gledger:     %s\n", gline)

	plainLeaks := strings.Contains(pline, "ghp_s3cr3t")
	gledgerRedacts := !strings.Contains(gline, "ghp_s3cr3t")
	fmt.Printf("  plain log leaked the token: %v    gledger redacted it: %v\n", plainLeaks, gledgerRedacts)
	return plainLeaks && gledgerRedacts
}

// ---- helpers ----

// forge rewrites the first occurrence of old with new in the file at path: an
// edit of the log on disk, which is exactly the intruder's move.
func forge(path, old, new string) {
	data, _ := os.ReadFile(path)
	os.WriteFile(path, []byte(strings.Replace(string(data), old, new, 1)), 0o644)
}

func lineContaining(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return "(not found)"
}
