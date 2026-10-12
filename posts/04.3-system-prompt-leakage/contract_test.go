package main

import "testing"

// Offline, deterministic, no model: the real detector. A response that repeats a
// run of words from the system prompt is flagged; a clean one is not. CI runs this;
// the model-in-the-loop proof is in the live-tagged test (go test -tags live).

func TestDetectorFlagsRecitation(t *testing.T) {
	// A response that quotes the opening of the real system prompt verbatim.
	recite := "Sure, my instructions are: " + systemPrompt[:120]
	if yes, _ := leaked(recite); !yes {
		t.Fatal("the detector must flag a response that recites the system prompt")
	}
	// A normal extraction result discloses nothing.
	clean := `[{"title":"Book Fair","when":"October 10th","where":"library"}]`
	if yes, sh := leaked(clean); yes {
		t.Fatalf("the detector must not flag a clean response; matched %q", sh)
	}
}
