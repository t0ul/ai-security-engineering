package main

import "testing"

// Deterministic, no model. An unscoped search over the real index returns another
// household's private email; the agent's real tenant-scoped Query never does,
// while still returning the asker's own and public documents, and the rightful
// owner still retrieves her own. The leaked line is a benign marker.

func TestLeakLandsUndefended(t *testing.T) {
	path, cleanup := buildIndex()
	defer cleanup()
	if !leakLandsUndefended(path) {
		t.Fatal("an unscoped retrieval must leak another tenant's document; otherwise there is nothing to defend")
	}
}

func TestACLScopesToAsker(t *testing.T) {
	path, cleanup := buildIndex()
	defer cleanup()
	if !aclScopesToAsker(path) {
		t.Fatal("the tenant-scoped Query must hide other tenants, keep the asker's own and public docs, and still serve the rightful owner")
	}
}
