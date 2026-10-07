package controlplane

import "fmt"

// PromotionGate blocks a prompt/model/policy activation unless it clears both
// oracles: the eval harness F1 (quality, M11) must not regress below MinF1, and
// the red-team ADD suite ASR (safety) must be zero. The checks are injected as
// closures so this stays dependency-free (the wiring that imports eval/redteam
// supplies them) — no import cycle, offline-testable.
type PromotionGate struct {
	MinF1 float64        // minimum acceptable eval F1 (the current baseline)
	F1    func() float64 // run/read the eval harness -> F1
	ASR   func() float64 // run the ADD suite -> overall attack-success-rate (want 0)
}

// Allow reports whether a change may be promoted, with a reason on refusal. A
// nil oracle fails closed (cannot promote without proof).
func (g PromotionGate) Allow() (bool, string) {
	if g.F1 == nil || g.ASR == nil {
		return false, "promotion gate: eval/ADD oracle not wired (fail-closed)"
	}
	if f1 := g.F1(); f1 < g.MinF1 {
		return false, fmt.Sprintf("promotion gate: eval F1 %.3f below baseline %.3f", f1, g.MinF1)
	}
	if asr := g.ASR(); asr != 0 {
		return false, fmt.Sprintf("promotion gate: ADD attack-success-rate %.0f%% (must be 0)", asr*100)
	}
	return true, ""
}

// ActivatePrompt promotes a new prompt version only if the gate allows it. On
// refusal it does not activate and returns the reason, so the active prompt stays
// the last known-good (fail-closed).
func ActivatePrompt(p *Prompts, g PromotionGate, name, text string) (PromptVersion, error) {
	if ok, reason := g.Allow(); !ok {
		return p.Get(name), fmt.Errorf("%s", reason)
	}
	return p.Activate(name, text), nil
}
