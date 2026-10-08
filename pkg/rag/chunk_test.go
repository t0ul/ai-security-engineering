package rag

import (
	"strings"
	"testing"
)

func TestCleanNormalizesWhitespace(t *testing.T) {
	in := "Hello\r\n\r\n\r\n   world  \t here​.\n\n\n\nBye"
	got := Clean(in)
	if strings.Contains(got, "\r") || strings.Contains(got, "​") || strings.Contains(got, "\t") {
		t.Fatalf("control/zero-width/tab survived: %q", got)
	}
	if strings.Contains(got, "   ") || strings.Contains(got, "\n\n\n") {
		t.Fatalf("runs not collapsed: %q", got)
	}
	if !strings.Contains(got, "world here") {
		t.Fatalf("words/spacing mangled: %q", got)
	}
}

func TestParagraphChunker(t *testing.T) {
	text := "Para one is short.\n\nPara two is also short.\n\n" + strings.Repeat("x", 700)
	chunks := ParagraphChunker{MaxChars: 100}.Chunk(text)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple paragraph chunks, got %d", len(chunks))
	}
	// the 700-char paragraph must stand on its own (exceeds max).
	last := chunks[len(chunks)-1]
	if len(last) < 500 {
		t.Fatalf("oversized paragraph should stand alone, got len %d", len(last))
	}
}

func TestFixedChunkerOverlap(t *testing.T) {
	text := strings.Repeat("abcdefghij", 60) // 600 chars
	chunks := FixedChunker{Size: 200, Overlap: 50}.Chunk(text)
	if len(chunks) < 3 {
		t.Fatalf("expected >=3 windows, got %d", len(chunks))
	}
	for _, c := range chunks {
		if len([]rune(c)) > 200 {
			t.Fatalf("chunk exceeds size: %d", len([]rune(c)))
		}
	}
}

func TestChunkerByName(t *testing.T) {
	if ChunkerByName("paragraph").Name() != "paragraph" ||
		ChunkerByName("fixed").Name() != "fixed" ||
		ChunkerByName("nonsense").Name() != "whole" {
		t.Fatal("ChunkerByName resolution wrong")
	}
}

// TestChunkedAddRetrievesPassage: with a paragraph chunker, a query returns the
// focused chunk, not the whole document, and DocID is the base id.
func TestChunkedAddRetrievesPassage(t *testing.T) {
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.Chunker = ParagraphChunker{MaxChars: 80}
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
