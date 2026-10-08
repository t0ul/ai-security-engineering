// Package guard is the Module 4 Layer-1 defense: it neutralizes indirect prompt
// injection in untrusted email BEFORE extraction, and detects system-prompt
// leakage in model output afterward.
//
// Detection of injection markers and verbatim echo is delegated to gumpers (the
// fleet guardrails library) so the patterns live in one reusable place. This
// package owns only the email-ingestion *policy* that is specific to the
// watched-folder agent: strip content a human never sees (HTML comments,
// zero-width/bidi runs), then blank an injected line and the two payload lines
// that usually follow it. Layer 2 is the spotlighting / instruction-hierarchy
// prompt in the extractor; no single layer is complete.
package guard

import (
	"regexp"
	"sort"
	"strings"

	"github.com/t0ul/gumpers"
)

const blockMarker = "[blocked: injected instruction removed]"

var (
	reHTMLComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	reZeroWidth   = regexp.MustCompile(`[\x{200b}-\x{200f}\x{202a}-\x{202e}\x{2060}\x{feff}]`)
	injRail       = gumpers.InjectionRail()
)

// Sanitize strips hidden content and neutralizes injection-marker lines (and the
// next two non-blank lines, where an injected event's imperative and its date
// usually sit). It returns the cleaned text and the sorted, unique finding keys.
func Sanitize(text string) (string, []string) {
	findings := map[string]bool{}

	if reHTMLComment.MatchString(text) {
		findings["hidden-html-comment"] = true
	}
	cleaned := reHTMLComment.ReplaceAllString(text, " ")

	if reZeroWidth.MatchString(cleaned) {
		findings["zero-width-chars"] = true
	}
	cleaned = reZeroWidth.ReplaceAllString(cleaned, "")

	lines := strings.Split(cleaned, "\n")
	out := make([]string, 0, len(lines))
	for i := 0; i < len(lines); {
		if hits := injRail.Check(lines[i]); len(hits) > 0 {
			for _, h := range hits {
				findings[h.Rule] = true
			}
			out = append(out, blockMarker)
			blanked, j := 0, i+1
			for j < len(lines) && blanked < 2 {
				if strings.TrimSpace(lines[j]) != "" {
					out = append(out, blockMarker)
					blanked++
				} else {
					out = append(out, lines[j])
				}
				j++
			}
			i = j
		} else {
			out = append(out, lines[i])
			i++
		}
	}

	keys := make([]string, 0, len(findings))
	for k := range findings {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(out, "\n"), keys
}

// DetectPromptLeak reports whether model output echoes its own system prompt
// (LLM07), via gumpers' verbatim-echo detector. The returned snippet is secret:
// log the boolean, never the snippet.
func DetectPromptLeak(output, systemPrompt string) (bool, string) {
	return gumpers.DetectEcho(output, systemPrompt, 6)
}
