// Command main is the runnable demo for lesson 05.1, scrubbing secrets and PII at
// ingest.
//
// Run it with:
//
//	go run .
//
// The agent indexes every email into a searchable corpus so it can answer
// questions later. That corpus is on disk and broadly queryable, so anything
// secret that lands in it sits there in the clear, waiting for a leak. This demo
// indexes an email full of secrets with no scrubbing, finds them all on disk, then
// scrubs at ingest with the real goflage recognizer and finds them gone. It is
// deterministic, no model involved. It is also honest about one secret the
// recognizer does not know.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/t0ul/goflage"
)

// leaky is the email full of secrets, read from attack/leaky.txt so what the
// chapter prints is what gets indexed. There is no embedded copy to drift from it.
func leaky() string {
	b, err := os.ReadFile("attack/leaky.txt")
	if err != nil {
		panic(err)
	}
	return string(b)
}

// recognized are the secrets goflage knows how to find, in the order the email
// carries them. The control should keep every one of these out of the corpus.
var recognized = []struct{ name, secret string }{
	{"parent email", "parent.contact@example.com"},
	{"SSN", "123-45-6789"},
	{"AWS key", "AKIAIOSFODNN7EXAMPLE"},
	{"GitHub token", "ghp_abcdefghijklmnopqrstuvwxyz0123456789"},
}

// unusual is a billing-key format goflage does not recognize. It is here to keep
// the lesson honest about what a pattern-based scrubber can and cannot promise.
const unusual = "sk-live-abc123DEF456ghi789"

func main() {
	dir, err := os.MkdirTemp("", "lesson-05.1-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	leak := indexRaw(dir)
	scrub := indexScrubbed(dir)

	fmt.Println()
	if leak && scrub {
		fmt.Println("attack blocked (the receipt): every recognized secret kept out of the corpus")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- the corpus on disk ----

// corpus writes one indexed document to a file on disk, the way the real retrieval
// index persists what the agent has read.
func corpus(dir string, text string) string {
	path := filepath.Join(dir, "corpus.txt")
	f, _ := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	defer f.Close()
	f.WriteString(text)
	return path
}

func onDisk(path, needle string) bool {
	data, _ := os.ReadFile(path)
	return strings.Contains(string(data), needle)
}

// ---- before: index the email as-is ----

func indexRaw(dir string) bool {
	fmt.Println("== before: index the email as-is ==")
	path := corpus(dir, leaky())
	all := true
	for _, r := range recognized {
		hit := onDisk(path, r.secret)
		fmt.Printf("  %-13s on disk: %v\n", r.name, hit)
		all = all && hit
	}
	fmt.Println("  every secret the email carried is now in the searchable corpus, in the clear")
	return all // the attack lands: all recognized secrets reached disk
}

// ---- after: scrub at ingest ----

func indexScrubbed(dir string) bool {
	fmt.Println("\n== after: scrub at ingest with goflage ==")
	clean, findings := goflage.New().Scrub(leaky())
	fmt.Printf("  goflage findings: %v\n", findings)
	path := corpus(dir, clean)

	none := true
	for _, r := range recognized {
		hit := onDisk(path, r.secret)
		fmt.Printf("  %-13s on disk: %v\n", r.name, hit)
		none = none && !hit
	}
	fmt.Printf("  what the corpus holds instead: %s\n", firstLine(clean))

	// The honest part: a format the recognizer does not know still slips through.
	fmt.Printf("\n  but the unrecognized billing key is still on disk: %v\n", onDisk(path, unusual))
	fmt.Println("  goflage is deterministic and high-precision; it redacts shapes it knows and")
	fmt.Println("  no others. An unusual token format is a recognizer it does not have yet.")
	return none // the control holds for every recognized secret
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// leakyScrubbed exposes the scrub result for the characterization test.
func leakyScrubbed() (string, []goflage.Finding) { return goflage.New().Scrub(leaky()) }
