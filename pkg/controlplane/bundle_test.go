package controlplane

import "testing"

// TestBundleSnapshotApply is the C10 invariant: a snapshot of the whole plane,
// applied after a chain of changes, restores every knob class at once — now
// including budgets (the dimension the bundle used to omit).
func TestBundleSnapshotApply(t *testing.T) {
	p := NewPrompts(map[string]string{"extractor": "DEFAULT"})
	s := NewSampling(map[string]SamplingConfig{"extractor": {Temperature: 0.1, MaxTokens: 900}})
	pol := NewPolicies(map[string][]string{"egress": {"a.com"}})
	bud := NewBudgets(map[string]BudgetConfig{"api": {RatePerMin: 60}})

	good := Snapshot("good", p, s, pol, bud) // capture the known-good defaults

	// Drift every knob.
	p.Activate("extractor", "BROKEN")
	s.Activate("extractor", SamplingConfig{Temperature: 9, MaxTokens: 1})
	pol.Activate("egress", []string{"evil.com"})
	bud.Activate("api", BudgetConfig{RatePerMin: 100000})

	good.Apply(p, s, pol, bud) // one-step rollback

	if p.Text("extractor") != "DEFAULT" {
		t.Errorf("prompt not rolled back: %q", p.Text("extractor"))
	}
	if c := s.Config("extractor"); c.Temperature != 0.1 || c.MaxTokens != 900 {
		t.Errorf("sampling not rolled back: %+v", c)
	}
	if items := pol.Items("egress"); len(items) != 1 || items[0] != "a.com" {
		t.Errorf("policy not rolled back: %v", items)
	}
	if c := bud.Config("api"); c.RatePerMin != 60 {
		t.Errorf("budget not rolled back: %+v", c)
	}
}
