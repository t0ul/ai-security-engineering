// Package pipeline is the pure, testable core of the watched-folder agent.
//
// It reads one email, neutralizes indirect injection (guard, M4), runs the
// registered tools under one trace_id, enforces each tool's declared capability
// (least privilege, M6 — a read-only tool cannot write an artifact), writes
// inert artifacts to the outbox, and records every step to a tamper-evident,
// value-redacting audit log (gledger, replacing the Python telemetry spine).
// The watcher daemon is a thin wrapper that calls ProcessEmail per dropped file.
package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/t0ul/ai-security-engineering/agent/guard"
	"github.com/t0ul/ai-security-engineering/agent/tool"
	"github.com/t0ul/ai-security-engineering/provenance"
	"github.com/t0ul/gledger"
)

// MaxBytes is the oversized-input DoS guard for the long-running agent (M10).
const MaxBytes = 256 * 1024

// Summary is the result of processing one email.
type Summary struct {
	TraceID   string   `json:"trace_id"`
	Source    string   `json:"source"`
	Events    int      `json:"events"`
	Artifacts []string `json:"artifacts"`
	Warnings  []string `json:"warnings"`
	Rejected  string   `json:"rejected,omitempty"`
	Bytes     int64    `json:"bytes,omitempty"`
}

// Pipeline wires a tool registry to an audit log and an outbox.
type Pipeline struct {
	Registry    *tool.Registry
	Audit       *gledger.AuditLog
	OutboxDir   string
	ToolNames   []string // defaults to ["event_extractor"]
	DefaultYear int      // defaults to 2026
	// Index, if set, persists each email into the retrieval corpus (RAG) as
	// untrusted provenance. The raw file archived to processed/ stays the source
	// of truth; the index is rebuildable derived state. Indexing failures are
	// audited, never fatal.
	Index func(traceID, source, rawText string) error
	// Signer, if set, writes a provenance content credential (<artifact>.sig)
	// next to each emitted artifact, so a consumer can verify the agent produced
	// it unaltered (M20 output authenticity).
	Signer *provenance.Signer
}

// ProcessEmail runs the full pipeline for one file.
func (p *Pipeline) ProcessEmail(path string) (Summary, error) {
	trace := gledger.NewTraceID()
	source := filepath.Base(path)
	if err := os.MkdirAll(p.OutboxDir, 0o755); err != nil {
		return Summary{}, err
	}
	p.Audit.Emit(trace, "request", "start", gledger.F{"source": source})

	fi, err := os.Stat(path)
	if err != nil {
		return Summary{}, err
	}
	if fi.Size() > MaxBytes {
		p.Audit.Emit(trace, "ingest", "rejected", gledger.F{"reason": "oversized", "bytes": fi.Size(), "limit": MaxBytes})
		p.Audit.Emit(trace, "request", "end", gledger.F{"status": "rejected"})
		return Summary{TraceID: trace, Source: source, Rejected: "oversized", Bytes: fi.Size()}, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return Summary{}, err
	}
	text := string(raw)

	// Persist the raw (untrusted) email into the retrieval corpus before any
	// sanitization — the corpus holds the real content; defense is applied at
	// retrieval time (rag.Assemble). The on-disk copy remains the backup.
	if p.Index != nil {
		if err := p.Index(trace, source, text); err != nil {
			p.Audit.Emit(trace, "index", "error", gledger.F{"source": source, "error": err.Error()})
		} else {
			p.Audit.Emit(trace, "index", "stored", gledger.F{"source": source, "bytes": len(text)})
		}
	}

	// M4: neutralize indirect prompt injection in the untrusted body, and record
	// what was found — the audit trail is the detection surface.
	text, findings := guard.Sanitize(text)
	if len(findings) > 0 {
		p.Audit.Emit(trace, "injection_guard", "detected", gledger.F{"indicators": findings})
	}

	names := p.ToolNames
	if len(names) == 0 {
		names = []string{"event_extractor"}
	}
	year := p.DefaultYear
	if year == 0 {
		year = 2026
	}
	stem := strings.TrimSuffix(source, filepath.Ext(source))
	summary := Summary{TraceID: trace, Source: source, Artifacts: []string{}, Warnings: []string{}}

	for _, name := range names {
		t, ok := p.Registry.Get(name)
		if !ok {
			continue
		}
		sp := p.Audit.Start(trace, name, gledger.F{"capability": string(t.Capability())})
		res := t.Run(text, tool.Ctx{Source: source, DefaultYear: year})
		sp.End()

		// Capability enforcement (M6): only a WRITE_ICS tool may emit artifacts.
		artifacts := res.Artifacts
		if len(artifacts) > 0 && res.Capability != tool.WriteICS {
			keys := make([]string, 0, len(artifacts))
			for k := range artifacts {
				keys = append(keys, k)
			}
			p.Audit.Emit(trace, name, "capability_violation",
				gledger.F{"declared": string(res.Capability), "tried_to_write": keys})
			artifacts = nil
		}

		for fname, content := range artifacts {
			outPath := filepath.Join(p.OutboxDir, stem+"."+fname)
			if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
				p.Audit.Emit(trace, name, "artifact_error", gledger.F{"file": fname, "error": err.Error()})
				continue
			}
			summary.Artifacts = append(summary.Artifacts, outPath)
			p.Audit.Emit(trace, name, "artifact_written",
				gledger.F{"file": filepath.Base(outPath), "bytes": len(content)})

			if p.Signer != nil {
				mark, _ := json.Marshal(p.Signer.Sign([]byte(content)))
				if err := os.WriteFile(outPath+".sig", mark, 0o644); err == nil {
					p.Audit.Emit(trace, name, "artifact_signed", gledger.F{"file": filepath.Base(outPath) + ".sig"})
				}
			}
		}

		summary.Events += len(res.Events)
		summary.Warnings = append(summary.Warnings, res.Warnings...)
		p.Audit.Emit(trace, name, "result", gledger.F{"events": len(res.Events), "warnings": res.Warnings})
	}

	p.Audit.Emit(trace, "request", "end",
		gledger.F{"status": "ok", "events": summary.Events, "artifacts": len(summary.Artifacts)})
	return summary, nil
}
