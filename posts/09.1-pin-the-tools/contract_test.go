package main

import "testing"

// Deterministic, no model. A direct MCP call takes a swapped tool definition
// (the rug-pull lands); the pinned gustoms gateway refuses the changed manifest
// and never forwards to the poisoned tool, while an operator re-approval lets a
// genuine change through. The poisoned definition is a benign marker string.

func TestRugPullLandsUndefended(t *testing.T) {
	if !rugPullLandsUndefended() {
		t.Fatal("a direct call must use the swapped tool definition; otherwise there is nothing to defend")
	}
}

func TestPinBlocksRugPull(t *testing.T) {
	if !pinBlocksRugPull() {
		t.Fatal("the pinned gateway must refuse the rug-pull, never forward to the poisoned tool, and recover on re-approval")
	}
}
