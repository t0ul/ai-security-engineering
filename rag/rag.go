// Package rag is the M8 retrieval layer hardened against an untrusted corpus:
// per-tenant access control on retrieval (no cross-tenant leakage), provenance
// tags on every document, and retrieval sanitization — untrusted chunks are
// injection-neutralized and XML-encapsulated (spotlighting) before they can
// reach a model, so a poisoned document is data, never instructions.
//
// Matching is deterministic keyword overlap (no embeddings; embedding-inversion
// and vector weaknesses are tracked separately), keeping the security behavior
// testable without a model.
package rag

import (
	"fmt"
	"sort"
	"strings"

	"github.com/t0ul/ai-security-engineering/agent/guard"
)

// Provenance marks whether a document's text is trusted or attacker-influenced.
type Provenance string

const (
	Trusted   Provenance = "trusted"
	Untrusted Provenance = "untrusted"
)

// Doc is one indexed document. Tenant "" is public (visible to all tenants).
type Doc struct {
	ID     string
	Tenant string
	Text   string
	Prov   Provenance
}

// Chunk is a retrieval hit.
type Chunk struct {
	DocID string
	Prov  Provenance
	Text  string
}

// Store is an in-memory corpus.
type Store struct{ docs []Doc }

// Add indexes a document.
func (s *Store) Add(d Doc) { s.docs = append(s.docs, d) }

// Query returns up to k chunks the tenant may see, ranked by keyword overlap.
// Authorization is enforced here, outside any model: a tenant sees only its own
// documents and public ones.
func (s *Store) Query(tenant, query string, k int) []Chunk {
	q := terms(query)
	type scored struct {
		c     Chunk
		score int
	}
	var hits []scored
	for _, d := range s.docs {
		if d.Tenant != "" && d.Tenant != tenant {
			continue // ACL: not this tenant's document
		}
		if n := overlap(q, terms(d.Text)); n > 0 {
			hits = append(hits, scored{Chunk{DocID: d.ID, Prov: d.Prov, Text: d.Text}, n})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]Chunk, 0, k)
	for i := 0; i < len(hits) && i < k; i++ {
		out = append(out, hits[i].c)
	}
	return out
}

// Assemble renders chunks for a prompt. Every chunk is XML-encapsulated with its
// provenance; untrusted chunks are first run through the injection guard, so
// hidden instructions in a poisoned document are neutralized before the model
// ever sees them.
func Assemble(chunks []Chunk) string {
	var b strings.Builder
	for _, c := range chunks {
		text := c.Text
		if c.Prov != Trusted {
			text, _ = guard.Sanitize(text)
		}
		fmt.Fprintf(&b, "<retrieved_context doc=%q provenance=%q>\n%s\n</retrieved_context>\n", c.DocID, string(c.Prov), text)
	}
	return b.String()
}

func terms(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		out[strings.Trim(w, ".,:;!?\"'()")] = true
	}
	return out
}

func overlap(a, b map[string]bool) int {
	n := 0
	for w := range a {
		if b[w] {
			n++
		}
	}
	return n
}
