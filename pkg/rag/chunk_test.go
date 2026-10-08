package rag

import (
	"strings"
	"testing"

	"github.com/t0ul/GoRag"
)

// TestChunkedAddRetrievesPassage: with a GoRag paragraph chunker, a query returns the
// focused chunk, not the whole document, and DocID is the base id. (The chunker/clean
// unit tests live in the GoRag module.)
func TestChunkedAddRetrievesPassage(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Chunker = gorag.ParagraphChunker{MaxChars: 80}
	doc := "The book fair is on Friday in the library.\n\nThe nurse needs updated immunization forms by October.\n\nPicture day is next week."
	if err := s.Add(Doc{ID: "news.txt", Tenant: "", Text: doc, Prov: Untrusted}); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Query("public", "nurse immunization forms", 1)
	if err != nil || len(hits) == 0 {
		t.Fatalf("expected a hit, got %d err %v", len(hits), err)
	}
	if hits[0].DocID != "news.txt" {
		t.Fatalf("DocID should be the base id, got %q", hits[0].DocID)
	}
	if strings.Contains(hits[0].Text, "book fair") {
		t.Fatalf("hit should be the focused chunk, not the whole doc: %q", hits[0].Text)
	}
}
