package main

import (
	"strings"
	"testing"
)

// Deterministic, no model. The four deterministic integrity checks each catch
// their class of mangled data, and the naive title cleaner really does invert a
// negation (the RED is real, not arranged).

func TestChecksCatchAllFour(t *testing.T) {
	if !checksCatchAll() {
		t.Fatal("every deterministic integrity check must flag its mangled input")
	}
}

func TestNaivePathInvertsNegation(t *testing.T) {
	got := naiveStrip("We do not accept late permission forms")
	if strings.Contains(strings.ToLower(got), "not") {
		t.Fatalf("the naive cleaner should drop the negation (that is the bug the real normalizer avoids), got %q", got)
	}
	if !strings.HasPrefix(strings.ToLower(got), "accept") {
		t.Fatalf("dropping the negation should invert the meaning to 'accept ...', got %q", got)
	}
}
