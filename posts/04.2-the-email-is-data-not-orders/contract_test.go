package main

import (
	"strings"
	"testing"
)

// Offline, deterministic, no model: the control's structural property. The data
// channel confines the untrusted email inside the untrusted delimiters and keeps
// the instructions in the system message, which is what the live test then shows
// the model actually heeding. CI runs this; the model-in-the-loop proof is in the
// live-tagged test (go test -tags live).

func TestDataChannelConfinesTheEmail(t *testing.T) {
	user := channelUser(hostile())
	if !strings.HasPrefix(strings.TrimSpace(user), "<untrusted_email>") ||
		!strings.Contains(user, "</untrusted_email>") {
		t.Fatal("the email must be enclosed in the untrusted delimiters")
	}
	if !strings.Contains(user, "Principal's Gala") {
		t.Fatal("the attack is left IN the channel on purpose; this lesson demotes it, it does not strip it")
	}
	if strings.Contains(channelPrompt, "Principal's Gala") || strings.Contains(naivePrompt, "untrusted_email") {
		t.Fatal("the instruction channel (system message) must not carry the untrusted email")
	}
}
