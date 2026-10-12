//go:build live

// Live proof against the local model from post 00.2. Excluded from offline
// `go test` and GitHub CI (no model there). Run with: go test -tags live -v ./...
// Must fail if the model is down, never skip.
package main

import (
	"context"
	"strings"
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/emaildrop/pkg/rag"
)

func TestPoisonSteersRawBlockedWhenEncapsulated(t *testing.T) {
	ctx := context.Background()
	chunks := corpus()

	// Concatenated raw, the planted instruction steers the answer at least once.
	rawUser := question + "\n\nRecords:\n" + naiveContext(chunks)
	landed := false
	for i := 0; i < 4 && !landed; i++ {
		out, err := chat(ctx, naivePrompt, rawUser)
		if err != nil {
			t.Fatalf("model unreachable (bring up the lab from post 00.2): %v", err)
		}
		landed = marker(out)
	}
	if !landed {
		t.Fatal("the undefended model should follow the poisoned document; otherwise there is nothing to defend")
	}

	// Encapsulated as untrusted retrieved_context and neutralized, it does not.
	safeUser := question + "\n" + rag.Assemble(chunks)
	for i := 0; i < 4; i++ {
		out, err := chat(ctx, controlplane.ChatSystemPrompt, safeUser)
		if err != nil {
			t.Fatalf("model unreachable: %v", err)
		}
		if marker(out) {
			t.Fatalf("the poison survived the retrieved-context channel on run %d: %s", i, strings.TrimSpace(out))
		}
	}
}
