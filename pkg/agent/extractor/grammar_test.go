package extractor

import "testing"

// Internal test: the governed grammar override (set from the Grammars plane's
// OnActivate) must be what the extractor actually sends in JSON mode; "" falls
// back to the shipped default. Exercises the real payload builder — no mocks.
func TestGovernedGrammarDrivesPayload(t *testing.T) {
	SetJSONMode(true)
	defer SetJSONMode(false)
	defer SetGrammar("")

	SetGrammar("CUSTOM-GBNF")
	if g := extractionPayload("hi")["grammar"]; g != "CUSTOM-GBNF" {
		t.Fatalf("governed grammar not used in payload: %v", g)
	}
	SetGrammar("") // unset → shipped default
	if g := extractionPayload("hi")["grammar"]; g != EventArrayGBNF {
		t.Fatalf("empty override should fall back to EventArrayGBNF, got: %v", g)
	}
}
