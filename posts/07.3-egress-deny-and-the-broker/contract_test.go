package main

import "testing"

// Deterministic, no model. A naive broker tunnels to an internal service; the real
// broker refuses a target that is not on the egress allowlist and reaches only one
// that is.

func TestNaiveBrokerReachesInternal(t *testing.T) {
	ok := startServers(t)
	if !naiveReachesInternal(ok.internal) {
		t.Fatal("the naive broker should dial the internal service; otherwise there is nothing to fix")
	}
}

func TestBrokerRefusesInternalAllowsAllowlisted(t *testing.T) {
	s := startServers(t)
	if !brokerGates(s.ok, s.internal) {
		t.Fatal("the broker must refuse the internal target and tunnel the allowlisted one")
	}
}
