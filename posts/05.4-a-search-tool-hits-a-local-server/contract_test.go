package main

import "testing"

// Deterministic, no model. A naive fetch tool reaches the machine's own internal
// service (SSRF lands); the egress policy refuses loopback and the metadata address
// while still allowing a legitimate allowlisted host.

func TestNaiveReachesInternalBlockedByPolicy(t *testing.T) {
	var hits int
	internal := newInternal(&hits)
	defer internal.Close()

	if !naiveReaches(internal.URL, &hits) {
		t.Fatal("the naive tool should reach the internal service; otherwise there is nothing to fix")
	}
	if !policyRefuses(internal.URL, &hits) {
		t.Fatal("the policy must refuse loopback and metadata while allowing the legit host")
	}
}

func TestPolicyAllowsLegitBlocksInternal(t *testing.T) {
	if policy.Check("http://search.example/q") != nil {
		t.Fatal("an allowlisted public host must be permitted")
	}
	if policy.Check("http://127.0.0.1:9/x") == nil {
		t.Fatal("loopback must be refused")
	}
	if policy.Check("http://169.254.169.254/latest/meta-data/") == nil {
		t.Fatal("the cloud-metadata address must be refused")
	}
	if policy.Check("http://rebind.example/") == nil {
		t.Fatal("an allowlisted host that resolves to loopback (DNS rebinding) must be refused")
	}
}
