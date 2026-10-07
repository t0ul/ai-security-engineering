package extractor

import "testing"

func TestActivePromptOverride(t *testing.T) {
	if ActivePrompt() != ExtractionPrompt {
		t.Fatal("default ActivePrompt should be the shipped ExtractionPrompt")
	}
	SetExtractionPrompt("CANDIDATE PROMPT")
	if ActivePrompt() != "CANDIDATE PROMPT" {
		t.Fatal("override should take effect")
	}
	SetExtractionPrompt("") // reset
	if ActivePrompt() != ExtractionPrompt {
		t.Fatal("empty override should reset to the default")
	}
}
