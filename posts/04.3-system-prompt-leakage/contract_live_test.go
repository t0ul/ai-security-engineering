//go:build live

// Live proof against the local model from post 00.2. Excluded from offline
// `go test` and GitHub CI (no model there). Run with: go test -tags live -v ./...
// Must fail if the model is down, never skip.
package main

import (
	"context"
	"testing"
)

func TestPromptLeaksRawWithheldDefended(t *testing.T) {
	ctx := context.Background()

	// Undefended, the model recites its prompt at least once.
	leakedOnce := false
	for i := 0; i < 4 && !leakedOnce; i++ {
		if yes, _ := leaked(undefended(ctx)); yes {
			leakedOnce = true
		}
	}
	if !leakedOnce {
		t.Fatal("the undefended model should recite its system prompt; otherwise there is nothing to catch")
	}

	// Defended, the detector withholds it every time.
	for i := 0; i < 4; i++ {
		if yes, sh := leaked(defended(ctx)); yes {
			t.Fatalf("the system prompt leaked past the detector on run %d: matched %q", i, sh)
		}
	}
}
