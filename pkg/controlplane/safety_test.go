package controlplane

import (
	"context"
	"strings"
	"testing"
)

func TestSafetyGates(t *testing.T) {
	audit, _ := tempAudit(t)
	s := NewSafety(audit)
	cases := []struct {
		level      KillLevel
		req, tools bool
	}{
		{LevelNone, true, true},
		{LevelBlockTools, true, false},
		{LevelPause, false, false},
		{LevelHalt, false, false},
	}
	for _, c := range cases {
		s.Set("carol", c.level)
		if s.AllowRequest() != c.req || s.AllowToolExec() != c.tools {
			t.Errorf("%s: req=%v tools=%v, want req=%v tools=%v",
				c.level, s.AllowRequest(), s.AllowToolExec(), c.req, c.tools)
		}
	}
}

func TestOrchestratorBlockToolsRunsButSkipsSandbox(t *testing.T) {
	audit, _ := tempAudit(t)
	gw := fakeGateway(t, map[string]string{
		"planner": "- a\n- b\n- c\nCOMMAND: ls -la",
		"coder":   "# Report\nall good.",
	})
	defer gw.Close()

	s := NewSafety(audit)
	s.Set("carol", LevelBlockTools)
	// Interp is nil on purpose: at BlockTools the executor must not detonate.
	o := &Orchestrator{
		LLM:     &LLMClient{BaseURL: gw.URL, HTTP: gw.Client()},
		Audit:   audit,
		Approve: func(State) bool { return true },
		Safety:  s,
	}
	final, err := o.Run(context.Background(), "t", "history")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(final.ToolOutput, "blocked by safety") {
		t.Fatalf("expected tool execution blocked, got ToolOutput=%q", final.ToolOutput)
	}
	if final.FinalMarkdown == "" {
		t.Fatal("coder should still format a result at block-tools level")
	}
}

func TestOrchestratorPauseHalts(t *testing.T) {
	audit, _ := tempAudit(t)
	s := NewSafety(audit)
	s.Set("carol", LevelPause)
	o := &Orchestrator{Audit: audit, Safety: s} // no LLM needed; must halt first
	final, err := o.Run(context.Background(), "t", "history")
	if err != nil {
		t.Fatal(err)
	}
	if final.FinalMarkdown != haltMessage || final.ToolCommand != "" {
		t.Fatalf("pause must halt before work: %+v", final)
	}
}
