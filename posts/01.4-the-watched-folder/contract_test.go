package main

import "testing"

// The lesson's contract as tests. Deterministic: the ingestion limits are plain
// code, no model. Scenario A is the size cap and the settle delay; scenario B is
// the per-cycle cap. Both arms must hold.

func TestAttackLandsUndefendedAndBlockedDefended_SizeSettle(t *testing.T) {
	maxBytes, settle, _ := limits()
	if !sizeAndSettle(maxBytes, settle) {
		t.Fatal("naive scan should read the oversized and half-written files; the capped scan should refuse and skip them")
	}
}

func TestPerCycleCapBoundsWork(t *testing.T) {
	_, _, perCycle := limits()
	if !perCycleCap(perCycle) {
		t.Fatal("naive scan should process the whole flood; the capped scan should process only perCycle and defer the rest")
	}
}
