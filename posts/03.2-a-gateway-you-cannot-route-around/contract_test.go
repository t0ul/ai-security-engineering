package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Deterministic: no model. Each attack is a real HTTP request at a real gateway.
// The naive proxy lets the caller pick the host (SSRF) and has no caps (flood);
// gouncer refuses an unknown model (400), rate-limits a burst (429), and rejects
// an oversized body (413).

func fixtures(t *testing.T) (backend string, internal string, hits *int) {
	h := 0
	b := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"[]"}}]}`)
	}))
	t.Cleanup(b.Close)
	in := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		h++
		io.WriteString(w, "INTERNAL-SECRET")
	}))
	t.Cleanup(in.Close)
	return b.URL, in.URL, &h
}

func TestSSRFReachesInternalUndefendedBlockedDefended(t *testing.T) {
	backend, internal, hits := fixtures(t)
	if !ssrfBlocked(backend, internal, hits) {
		t.Fatal("naive proxy must reach the internal service; gouncer must refuse an unknown model and never contact it")
	}
}

func TestFloodBounded(t *testing.T) {
	backend, _, _ := fixtures(t)
	if !floodBounded(backend) {
		t.Fatal("naive proxy serves the whole burst; gouncer must return 429 past the rate cap")
	}
}

func TestBodyCapped(t *testing.T) {
	backend, _, _ := fixtures(t)
	if !bodyCapped(backend) {
		t.Fatal("naive proxy accepts the oversized body; gouncer must return 413")
	}
}
