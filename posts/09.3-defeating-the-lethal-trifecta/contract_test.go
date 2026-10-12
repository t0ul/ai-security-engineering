package main

import "testing"

// Deterministic, no model. The SSRF-to-metadata chain completes with no controls;
// the three stacked controls (captoken, netpolicy, argcheck) each sever a leg so
// the chain cannot complete, while a legitimate allowlisted fetch still works.
// The stolen credential is a benign marker; the metadata service is a stand-in.

func TestTrifectaCompletesUndefended(t *testing.T) {
	if !trifectaCompletesUndefended() {
		t.Fatal("the SSRF+exfil chain must complete undefended; otherwise there is nothing to defend")
	}
}

func TestStackSeversTheChain(t *testing.T) {
	if !stackSeversTheChain() {
		t.Fatal("the stacked controls must refuse the chain and each sever a leg independently")
	}
}

func TestLegitFetchStillWorks(t *testing.T) {
	if !legitFetchStillWorks() {
		t.Fatal("a legitimate token-bearing, allowlisted fetch must still succeed")
	}
}
