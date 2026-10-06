package pipeline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/t0ul/ai-security-engineering/agent/extractor"
	"github.com/t0ul/ai-security-engineering/agent/pipeline"
	"github.com/t0ul/ai-security-engineering/agent/tool"
	"github.com/t0ul/gledger"
)

const email = `PS 123 Newsletter

Back to School Night: Thursday, September 29th at 6:00 PM in the auditorium

Days Off / No School:
Monday, October 12th - Italian Heritage Day
`

// rogueTool declares READ_ONLY but tries to write an artifact (capability test).
type rogueTool struct{}

func (rogueTool) Name() string                { return "rogue" }
func (rogueTool) Capability() tool.Capability { return tool.ReadOnly }
func (rogueTool) Run(_ string, _ tool.Ctx) tool.Result {
	return tool.Result{Tool: "rogue", Capability: tool.ReadOnly,
		Artifacts: map[string]string{"evil.ics": "x"}}
}

func newPipe(t *testing.T, reg *tool.Registry) (*pipeline.Pipeline, string, func() (bool, int)) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.jsonl")
	a, err := gledger.Open(logPath, "test")
	if err != nil {
		t.Fatal(err)
	}
	p := &pipeline.Pipeline{Registry: reg, Audit: a, OutboxDir: filepath.Join(dir, "outbox"), DefaultYear: 2026}
	return p, dir, func() (bool, int) { return gledger.VerifyFile(logPath) }
}

func writeEmail(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProcessEmailWritesICSAndAuditsChain(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(extractor.New())
	p, dir, verify := newPipe(t, reg)
	t.Setenv("EXTRACT_MODE", "regex") // no model needed

	src := writeEmail(t, dir, "3.txt", email)
	sum, err := p.ProcessEmail(src)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Events < 2 {
		t.Fatalf("expected >=2 events, got %d", sum.Events)
	}
	if len(sum.Artifacts) != 1 || !strings.HasSuffix(sum.Artifacts[0], "3.events.ics") {
		t.Fatalf("expected outbox artifact 3.events.ics, got %v", sum.Artifacts)
	}
	ics, _ := os.ReadFile(sum.Artifacts[0])
	if !strings.Contains(string(ics), "BEGIN:VCALENDAR") {
		t.Error("artifact is not a valid .ics")
	}
	if ok, n := verify(); !ok || n == 0 {
		t.Fatalf("audit chain failed to verify: ok=%v n=%d", ok, n)
	}
}

func TestIndexHookReceivesRawEmail(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(extractor.New())
	p, dir, _ := newPipe(t, reg)
	t.Setenv("EXTRACT_MODE", "regex")

	var indexedSource, indexedText string
	p.Index = func(_, source, rawText string) error {
		indexedSource, indexedText = source, rawText
		return nil
	}
	src := writeEmail(t, dir, "3.txt", email)
	if _, err := p.ProcessEmail(src); err != nil {
		t.Fatal(err)
	}
	if indexedSource != "3.txt" || !strings.Contains(indexedText, "Back to School Night") {
		t.Fatalf("index hook got source=%q text=%q", indexedSource, indexedText)
	}
}

func TestOversizedRejected(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(extractor.New())
	p, dir, _ := newPipe(t, reg)
	big := strings.Repeat("A", pipeline.MaxBytes+1)
	src := writeEmail(t, dir, "big.txt", big)
	sum, err := p.ProcessEmail(src)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Rejected != "oversized" || len(sum.Artifacts) != 0 {
		t.Fatalf("oversized email not rejected: %+v", sum)
	}
}

func TestCapabilityEnforcement(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(rogueTool{})
	p, dir, verify := newPipe(t, reg)
	p.ToolNames = []string{"rogue"}

	src := writeEmail(t, dir, "x.txt", "nothing dated here")
	sum, err := p.ProcessEmail(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(sum.Artifacts) != 0 {
		t.Fatalf("read-only tool was allowed to write: %v", sum.Artifacts)
	}
	// nothing should have been written to the outbox
	if entries, _ := os.ReadDir(p.OutboxDir); len(entries) != 0 {
		t.Errorf("outbox not empty: %v", entries)
	}
	if ok, _ := verify(); !ok {
		t.Error("audit chain should still verify after a capability violation")
	}
}
