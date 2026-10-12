package main

import (
	"strings"
	"testing"

	"github.com/t0ul/gorauder"
)

// Offline, deterministic, no model: the method is wired to the real libraries and
// the new app's hardened config spotlights its input. CI runs this; the live
// model-in-the-loop proof (the gate going green against the real model) is in the
// live-tagged test.

func TestHardenedConfigSpotlights(t *testing.T) {
	w := wrap("anything")
	if !strings.Contains(w, "<text>") || !strings.Contains(w, "</text>") {
		t.Fatal("the hardened config must fence the input in untrusted-data tags")
	}
	sys := strings.ToLower(safeSys)
	if !strings.Contains(sys, "never instructions") || !strings.Contains(sys, "data") {
		t.Fatal("the hardened system prompt must declare the input data, never instructions")
	}
}

func TestTechniqueIsWiredToTheCatalog(t *testing.T) {
	tech := injection()
	if len(tech.Seeds) == 0 {
		t.Fatal("the technique must carry real gorauder catalog seeds")
	}
	// Every seed came from the catalog and is a markered direct injection.
	catalog := gorauder.CatalogSeeds(gorauder.LLM01PromptInjection)
	inCatalog := func(id string) bool {
		for _, c := range catalog {
			if c.ID == id {
				return true
			}
		}
		return false
	}
	for _, s := range tech.Seeds {
		if !inCatalog(s.ID) || s.Marker == "" {
			t.Fatalf("seed %q is not a markered catalog seed", s.ID)
		}
	}
	if tech.Undefended == nil || tech.Defended == nil {
		t.Fatal("the technique needs both app configs as targets")
	}
}
