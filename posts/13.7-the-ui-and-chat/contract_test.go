package main

import (
	"strings"
	"testing"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/emaildrop/pkg/rag"
)

// Offline, deterministic, no model: the chat surface reuses the retrieval control.
// rag.Assemble fences every chunk as <retrieved_context> with its provenance and
// neutralizes untrusted ones, and the governed chat prompt declares that channel
// to be unverified data the assistant must never obey. CI runs this; the model-in-
// the-loop proof is in the live-tagged test.

func TestChatContextIsEncapsulated(t *testing.T) {
	assembled := rag.Assemble(corpus())
	if !strings.Contains(assembled, "<retrieved_context") || !strings.Contains(assembled, `provenance="untrusted"`) {
		t.Fatal("retrieved chat context must be fenced as untrusted <retrieved_context>")
	}
	if !strings.Contains(assembled, "book fair") {
		t.Fatal("a genuine notice must still be present (the poison is demoted, not everything dropped)")
	}
	if strings.Contains(naiveContext(corpus()), "retrieved_context") {
		t.Fatal("the naive chat context must not encapsulate; that is the before/after")
	}
}

func TestGovernedChatPromptDeclaresBoundary(t *testing.T) {
	sys := strings.ToLower(controlplane.ChatSystemPrompt)
	if !strings.Contains(sys, "retrieved_context") || !strings.Contains(sys, "never follow") {
		t.Fatal("the governed chat prompt must tell the assistant to treat <retrieved_context> as data it must never obey")
	}
}
