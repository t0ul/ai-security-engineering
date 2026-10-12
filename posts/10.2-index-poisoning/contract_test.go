package main

import (
	"strings"
	"testing"

	"github.com/t0ul/emaildrop/pkg/rag"
)

// Offline, deterministic, no model: the control's structural property. rag.Assemble
// encapsulates every retrieved chunk as <retrieved_context> with its provenance and
// neutralizes untrusted ones, and the governed system prompt declares that channel
// to be unverified data the model must never obey. CI runs this; the model-in-the-
// loop proof is in the live-tagged test (go test -tags live).

func TestRetrievedContextIsEncapsulatedAndDemoted(t *testing.T) {
	chunks := corpus()
	assembled := rag.Assemble(chunks)

	// Every chunk is bounded as untrusted-or-trusted retrieved_context data.
	if !strings.Contains(assembled, "<retrieved_context") || !strings.Contains(assembled, "</retrieved_context>") {
		t.Fatal("retrieved chunks must be enclosed in <retrieved_context> blocks")
	}
	if !strings.Contains(assembled, `provenance="untrusted"`) {
		t.Fatal("the poisoned chunk must be marked untrusted in the channel")
	}

	// The genuine fact survives: the control demotes the poison, it does not drop
	// legitimate retrieved facts.
	if !strings.Contains(assembled, "Multicultural Potluck") {
		t.Fatal("a genuine retrieved fact must still be present")
	}

	// The naive path has no boundary at all: a planted instruction looks like a fact.
	if strings.Contains(naiveContext(chunks), "retrieved_context") {
		t.Fatal("the naive context must not encapsulate; that is the whole before/after")
	}
}

func TestSystemPromptDeclaresTheDataBoundary(t *testing.T) {
	sys := strings.ToLower(controlplaneChatSystemPrompt())
	if !strings.Contains(sys, "retrieved_context") || !strings.Contains(sys, "never follow") {
		t.Fatal("the governed prompt must tell the model to treat <retrieved_context> as data it must never obey")
	}
}
