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
