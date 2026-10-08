package controlplane

// Known-good bundle (C10): an atomic snapshot of the whole governed plane —
// every prompt, sampling config, policy allowlist, and budget at their current
// active values — that the operator can roll the entire plane back to in one step. The
// ties-it-together capstone of the governed-knob set: when a change (or a chain
// of changes) goes wrong, restore a labeled safe point instead of reverting each
// knob by hand.

// Bundle is a captured configuration across all governed knob classes.
type Bundle struct {
	Label     string                     `json:"label"`
	Prompts   map[string]string          `json:"prompts"`
	Sampling  map[string]SamplingConfig  `json:"sampling"`
	Policies  map[string][]string        `json:"policies"`
	Budgets   map[string]BudgetConfig    `json:"budgets,omitempty"`
	Retrieval map[string]RetrievalConfig `json:"retrieval,omitempty"`
	Grammars  map[string]string          `json:"grammars,omitempty"`
	Models    map[string]string          `json:"models,omitempty"`
}

// Snapshot captures the current active (or shipped-default) value of every knob
// across the resolvers into a labeled bundle. Nil resolvers are skipped.
func Snapshot(label string, p *Prompts, s *Sampling, pol *Policies, bud *Budgets, ret *Retrieval, gram *Grammars, mod *Models) Bundle {
	b := Bundle{Label: label, Prompts: map[string]string{}, Sampling: map[string]SamplingConfig{}, Policies: map[string][]string{}, Budgets: map[string]BudgetConfig{}, Retrieval: map[string]RetrievalConfig{}, Grammars: map[string]string{}, Models: map[string]string{}}
	if p != nil {
		for _, v := range p.List() {
			b.Prompts[v.Name] = v.Text
		}
	}
	if s != nil {
		for _, v := range s.List() {
			b.Sampling[v.Name] = v.Config
		}
	}
	if pol != nil {
		for _, v := range pol.List() {
			b.Policies[v.Name] = v.Items
		}
	}
	if bud != nil {
		for _, v := range bud.List() {
			b.Budgets[v.Name] = v.Config
		}
	}
	if ret != nil {
		for _, v := range ret.List() {
			b.Retrieval[v.Name] = v.Config
		}
	}
	if gram != nil {
		for _, v := range gram.List() {
			b.Grammars[v.Name] = v.Text
		}
	}
	if mod != nil {
		for _, v := range mod.List() {
			b.Models[v.Name] = v.Model
		}
	}
	return b
}

// Apply restores every knob in the bundle by activating it — so the rollback is
// itself a governed, audited, persisted set of activations (each resolver's
// OnActivate fires, driving the extractor prompt/sampling hooks too). Nil
// resolvers are skipped.
func (b Bundle) Apply(p *Prompts, s *Sampling, pol *Policies, bud *Budgets, ret *Retrieval, gram *Grammars, mod *Models) {
	if p != nil {
		for name, text := range b.Prompts {
			p.Activate(name, text)
		}
	}
	if s != nil {
		for name, cfg := range b.Sampling {
			s.Activate(name, cfg)
		}
	}
	if pol != nil {
		for name, items := range b.Policies {
			pol.Activate(name, items)
		}
	}
	if bud != nil {
		for name, cfg := range b.Budgets {
			bud.Activate(name, cfg)
		}
	}
	if ret != nil {
		for name, cfg := range b.Retrieval {
			ret.Activate(name, cfg)
		}
	}
	if gram != nil {
		for name, text := range b.Grammars {
			gram.Activate(name, text)
		}
	}
	if mod != nil {
		for name, model := range b.Models {
			mod.Activate(name, model)
		}
	}
}
