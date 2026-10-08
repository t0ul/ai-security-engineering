package extractor

import (
	"os"
	"testing"
)

func TestExtractionModelBinding(t *testing.T) {
	os.Unsetenv("EXTRACTOR_MODEL")
	os.Unsetenv("PLANNER_MODEL")
	SetModel("")
	if got := extractionModel(); got != "planner" {
		t.Fatalf("default model should be planner, got %q", got)
	}
	SetModel("extractor") // logical binding — swap = config, not code
	if got := extractionModel(); got != "extractor" {
		t.Fatalf("override should bind the logical model, got %q", got)
	}
	SetModel("")
}

func TestExtractionPayloadModelAndGrammar(t *testing.T) {
	SetModel("extractor")
	defer SetModel("")
	SetJSONMode(true)
	defer SetJSONMode(false)

	p := extractionPayload("some email")
	if p["model"] != "extractor" {
		t.Fatalf("payload model = %v, want extractor", p["model"])
	}
	if _, ok := p["grammar"]; !ok {
		t.Fatal("json mode should attach a GBNF grammar for valid-by-construction output")
	}

	SetJSONMode(false)
	if _, ok := extractionPayload("x")["grammar"]; ok {
		t.Fatal("grammar should be absent when json mode is off")
	}
}
