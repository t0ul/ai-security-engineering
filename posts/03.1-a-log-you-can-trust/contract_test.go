package main

import "testing"

// Deterministic: no model. The undefended plain log cannot detect a tamper (the
// attack lands); gledger's hash chain does (the attack is blocked); and gledger
// redacts a secret the plain log writes in the clear.

func TestTamperGoesUnnoticedUndefended(t *testing.T) {
	dir := t.TempDir()
	if !tamperGoesUnnoticed(dir) {
		t.Fatal("a plain log should accept the forged line undetected; otherwise there is nothing to fix")
	}
}

func TestTamperIsCaughtDefended(t *testing.T) {
	dir := t.TempDir()
	if !tamperIsCaught(dir) {
		t.Fatal("gledger's chain must detect the tamper (chain_ok=false)")
	}
}

func TestSecretIsRedacted(t *testing.T) {
	dir := t.TempDir()
	if !secretIsRedacted(dir) {
		t.Fatal("gledger must redact the token the plain log writes in the clear")
	}
}
