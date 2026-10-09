package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/t0ul/ai-security-engineering/pkg/compaction"
	"github.com/t0ul/ai-security-engineering/pkg/gateway"
)

// TestCompactHistoryBudgets locks B1: a long chat history is collapsed into a running
// summary + the recent turns verbatim instead of overflowing the model's context window.
// The injected summarizer is the Compactor's designed seam (offline, no live model).
func TestCompactHistoryBudgets(t *testing.T) {
	summarize := func(ts []compaction.Turn) string { return fmt.Sprintf("summary-of-%d", len(ts)) }

	// 20 turns, each several words; a small budget forces compaction.
	var turns []gateway.Turn
	for i := 0; i < 20; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		turns = append(turns, gateway.Turn{Role: role, Content: fmt.Sprintf("turn number %d has several words here", i)})
	}

	out := compactHistory(turns, 20, summarize) // budget/2 = 10 tokens << total
	if len(out) >= len(turns) {
		t.Fatalf("over budget history must shrink: got %d from %d", len(out), len(turns))
	}
	// A trusted running summary leads the compacted history.
	if !strings.Contains(out[0].Content, "Summary of earlier trusted turns") {
		t.Errorf("first turn should be the running summary, got %q", out[0].Content)
	}
	// The 6 most-recent turns survive verbatim at the tail.
	last := out[len(out)-1]
	if last.Content != turns[len(turns)-1].Content {
		t.Errorf("most-recent turn must be kept verbatim: got %q want %q", last.Content, turns[len(turns)-1].Content)
	}

	// Under budget → unchanged (a big budget never compacts).
	if got := compactHistory(turns, 100000, summarize); len(got) != len(turns) {
		t.Errorf("under budget must be a no-op: got %d want %d", len(got), len(turns))
	}
	// No summarizer (e.g. offline) → unchanged, chat still works.
	if got := compactHistory(turns, 20, nil); len(got) != len(turns) {
		t.Errorf("nil summarizer must be a no-op: got %d want %d", len(got), len(turns))
	}
}

// TestCompactHistoryNoRawRoleLeak: the Compactor may emit a "tool" role for the
// quarantined untrusted summary; compactHistory must map it to a role the chat API
// speaks so it is never sent raw.
func TestCompactHistoryRoleMapping(t *testing.T) {
	summarize := func(ts []compaction.Turn) string { return "s" }
	var turns []gateway.Turn
	for i := 0; i < 20; i++ {
		turns = append(turns, gateway.Turn{Role: "user", Content: fmt.Sprintf("many words in turn %d here now", i)})
	}
	for _, tn := range compactHistory(turns, 20, summarize) {
		switch tn.Role {
		case "user", "assistant", "system":
		default:
			t.Errorf("unexpected role %q sent to the chat API", tn.Role)
		}
	}
}
