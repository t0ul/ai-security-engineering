package main

import (
	"strings"
	"testing"

	"github.com/t0ul/emaildrop/pkg/agent/guard"
)

// Offline, deterministic, no model: the real guard.Sanitize must detect the
// injected imperative and strip the attacker's event from the email before it ever
// reaches a model. This is the first layer of the control and it runs in CI.
// The full model-in-the-loop proof is in the live-tagged test (go test -tags live).

func TestGuardNeutralizesInjection(t *testing.T) {
	clean, findings := guard.Sanitize(hostile())

	found := false
	for _, f := range findings {
		if strings.Contains(f, "inject") {
			found = true
		}
	}
	if !found {
		t.Fatalf("guard should flag the injection; findings were %v", findings)
	}
	if marker(clean) {
		t.Fatalf("guard must strip the attacker's event from the email; sanitized text still contains it:\n%s", clean)
	}
}
