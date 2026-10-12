package main

import "testing"

// Deterministic, no model. A crafted item that states a weekday its date does not
// fall on lands on the wrong day in a view that trusts the text, and on its real
// day (with the mismatch flagged) in the agent's date-first view.

func TestStatedWeekdayMisplacesUndefended(t *testing.T) {
	if !misplacedUndefended() {
		t.Fatal("placing by the stated weekday must put the crafted item on the wrong day; otherwise there is nothing to fix")
	}
}

func TestDateFirstPlacesCorrectlyAndFlags(t *testing.T) {
	if !placedByDateDefended() {
		t.Fatal("placing by the parsed date must put the item on its real day and flag the weekday mismatch")
	}
}
