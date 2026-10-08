package controlplane

import (
	"context"

	"github.com/t0ul/gledger"
	"github.com/t0ul/goflage"
	"github.com/t0ul/gumpers"
)

// Middleware is the inbound sanitization boundary. It replaces the Python
// NeMo+Presidio stack with the fleet equivalents:
//   - gumpers input rails (injection detection, plus an optional topic denylist)
//     stand in for NeMo's semantic routing. A Block verdict halts the request.
//   - goflage scrubs PII and secrets (IP, email, SECRET_KEY shapes), the way
//     Presidio did, logging only entity types and counts — never the values.
//
// NeMo's LLM-based semantic classification becomes deterministic rails here;
// a model-backed topic classifier can be added later via an LLM-backed rail.
type Middleware struct {
	guard    *gumpers.Engine
	analyzer *goflage.Analyzer
	audit    *gledger.AuditLog
}

// NewMiddleware builds the sanitizer. topicTerms, if non-empty, installs a
// blocking denylist rail for out-of-scope topics (the agent is restricted to
// formatting terminal logs). The injection rail is always installed.
func NewMiddleware(audit *gledger.AuditLog, topicTerms ...string) *Middleware {
	opts := []gumpers.Option{
		gumpers.WithInputRail(gumpers.InjectionRail()),
		gumpers.WithAuditor(audit),
	}
	if len(topicTerms) > 0 {
		opts = append(opts, gumpers.WithInputRail(
			gumpers.DenylistRail("topic", gumpers.Block, topicTerms...)))
	}
	return &Middleware{
		guard:    gumpers.New(opts...),
		analyzer: goflage.New(),
		audit:    audit,
	}
}

// Sanitize runs the inbound rails then scrubs PII/secrets. It returns the
// log-safe text and whether a rail blocked the request. When blocked, the
// original text is returned unscrubbed and the caller must halt the loop.
func (m *Middleware) Sanitize(ctx context.Context, traceID, raw string) (safe string, blocked bool) {
	v := m.guard.CheckInput(traceID, raw)
	if v.Blocked {
		m.audit.Emit(traceID, "middleware", "blocked", gledger.F{"reasons": v.Reasons()})
		return raw, true
	}

	scrubbed, findings := m.analyzer.Scrub(raw)
	entities := make([]string, 0, len(findings))
	hits := 0
	for _, f := range findings {
		entities = append(entities, f.Entity)
		hits += f.Count
	}
	m.audit.Emit(traceID, "goflage", "scrub", gledger.F{"hits": hits, "entities": entities})
	return scrubbed, false
}
