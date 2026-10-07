// Package compaction keeps an agent's context under a token budget by summarizing
// (compacting) older turns into a running summary while keeping recent turns
// verbatim — the sliding-window + running-summary pattern a long-running agent
// needs so it never blows the context window.
//
// The security control is the point: the summary is *trusted context* on the next
// turn, so an attacker wants to write it. A naive compactor concatenates every
// evicted turn into one summary and feeds it back as trusted system context —
// which launders an injected instruction out of an untrusted email/web/RAG turn
// into trusted standing (summarization injection), or drops a safety-relevant
// trusted line into an untrusted blob. This compactor defends by **provenance
// partition**: trusted and untrusted turns are summarized separately, the
// untrusted summary stays provenance-untrusted and is run through a sanitizer
// (guard-shaped) so a laundered instruction is neutralized, and untrusted content
// is never promoted into a trusted summary. Stdlib only; the summarizer and
// sanitizer are injected so the package is offline-testable without an LLM.
package compaction

import "strings"

// Turn is one conversation entry. Trusted=false marks content that originated from
// an untrusted source (an email, a web fetch, a RAG chunk) and must never be
// promoted to trusted standing by compaction.
type Turn struct {
	Role    string `json:"role"` // "system" | "user" | "assistant" | "tool"
	Text    string `json:"text"`
	Trusted bool   `json:"trusted"`
}

// Summarizer condenses turns into a short summary string (an LLM call in prod; a
// deterministic stub in tests/ADD).
type Summarizer func([]Turn) string

// Sanitizer neutralizes injected instructions in text (guard.Sanitize-shaped:
// returns the cleaned text and any findings).
type Sanitizer func(string) (string, []string)

// Compactor applies the budgeted running-summary compaction.
type Compactor struct {
	MaxTokens   int              // compact once the total exceeds this
	KeepRecent  int              // most-recent turns kept verbatim
	Summarize   Summarizer       // required
	Sanitize    Sanitizer        // optional; applied to the UNTRUSTED summary
	CountTokens func(string) int // optional; defaults to a word count
}

func (c *Compactor) count(s string) int {
	if c.CountTokens != nil {
		return c.CountTokens(s)
	}
	return len(strings.Fields(s))
}

// Total is the current token estimate for turns.
func (c *Compactor) Total(turns []Turn) int {
	n := 0
	for _, t := range turns {
		n += c.count(t.Text)
	}
	return n
}

// Compact returns a compacted turn list when turns exceed MaxTokens: a trusted
// running-summary of the evicted trusted turns, a separate provenance-untrusted
// (and sanitized) summary of the evicted untrusted turns, then the KeepRecent
// most-recent turns verbatim. Under budget, or with no summarizer, it returns the
// input unchanged. The invariant: no untrusted evicted content ever appears in a
// Trusted=true turn.
func (c *Compactor) Compact(turns []Turn) []Turn {
	if c.Summarize == nil || c.Total(turns) <= c.MaxTokens || len(turns) <= c.KeepRecent {
		return turns
	}
	cut := len(turns) - c.KeepRecent
	if cut < 0 {
		cut = 0
	}
	older, recent := turns[:cut], turns[cut:]

	var trustedOld, untrustedOld []Turn
	for _, t := range older {
		if t.Trusted {
			trustedOld = append(trustedOld, t)
		} else {
			untrustedOld = append(untrustedOld, t)
		}
	}

	out := make([]Turn, 0, 2+len(recent))
	if len(trustedOld) > 0 {
		out = append(out, Turn{Role: "system", Trusted: true, Text: "Summary of earlier trusted turns: " + c.Summarize(trustedOld)})
	}
	if len(untrustedOld) > 0 {
		s := c.Summarize(untrustedOld)
		if c.Sanitize != nil {
			s, _ = c.Sanitize(s) // neutralize any laundered instruction
		}
		// Stays Trusted=false and visibly quarantined, so it re-enters context as
		// untrusted data, not as a standing instruction.
		out = append(out, Turn{Role: "tool", Trusted: false, Text: "<untrusted_summary>\n" + s + "\n</untrusted_summary>"})
	}
	return append(out, recent...)
}

// TrustedContext renders only the trusted turns — what would be handed to the
// model as system/standing context. Useful to assert that untrusted content never
// crossed the provenance boundary.
func TrustedContext(turns []Turn) string {
	var b strings.Builder
	for _, t := range turns {
		if t.Trusted {
			b.WriteString(t.Text)
			b.WriteString("\n")
		}
	}
	return b.String()
}
