package items

import "testing"

func TestDocumentType(t *testing.T) {
	if got := DocumentType("PS 51 Family Handbook\nArrival is at 8:15. Dismissal policy...\n"); got != "reference" {
		t.Fatalf("a handbook should classify reference, got %q", got)
	}
	if got := DocumentType("PTA Newsletter\nThe book fair is on October 2.\n"); got != "bulletin" {
		t.Fatalf("a newsletter should classify bulletin, got %q", got)
	}
}
