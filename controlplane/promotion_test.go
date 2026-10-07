package controlplane

import "testing"

func TestPromotionGateAllowsCleanChange(t *testing.T) {
	g := PromotionGate{MinF1: 0.87, F1: func() float64 { return 0.9 }, ASR: func() float64 { return 0 }}
	if ok, reason := g.Allow(); !ok {
		t.Fatalf("clean change should promote: %s", reason)
	}
}

func TestPromotionGateBlocksRegression(t *testing.T) {
	low := PromotionGate{MinF1: 0.87, F1: func() float64 { return 0.80 }, ASR: func() float64 { return 0 }}
	if ok, _ := low.Allow(); ok {
		t.Fatal("an F1 regression must be blocked")
	}
	leak := PromotionGate{MinF1: 0.87, F1: func() float64 { return 0.95 }, ASR: func() float64 { return 0.1 }}
	if ok, _ := leak.Allow(); ok {
		t.Fatal("a non-zero ASR must be blocked")
	}
}

func TestPromotionGateFailsClosedWithoutOracles(t *testing.T) {
	if ok, _ := (PromotionGate{MinF1: 0.87}).Allow(); ok {
		t.Fatal("no oracles -> must fail closed")
	}
}

func TestActivatePromptRespectsGate(t *testing.T) {
	p := NewPrompts(map[string]string{"planner": "DEFAULT"})
	blocked := PromotionGate{MinF1: 0.87, F1: func() float64 { return 0.5 }, ASR: func() float64 { return 0 }}
	if _, err := ActivatePrompt(p, blocked, "planner", "RISKY"); err == nil {
		t.Fatal("blocked gate must not activate")
	}
	if p.Get("planner").Text != "DEFAULT" {
		t.Fatal("active prompt must stay the known-good on a blocked promotion")
	}
	ok := PromotionGate{MinF1: 0.87, F1: func() float64 { return 0.9 }, ASR: func() float64 { return 0 }}
	if _, err := ActivatePrompt(p, ok, "planner", "GOOD"); err != nil {
		t.Fatalf("clean gate should activate: %v", err)
	}
	if p.Get("planner").Text != "GOOD" {
		t.Fatal("clean promotion should activate the new prompt")
	}
}
