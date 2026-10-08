package gateway

import (
	"strings"
	"testing"
	"time"
)

func TestTrustedDateBlock(t *testing.T) {
	// A Thursday; its week runs Mon 2026-10-05 .. Sun 2026-10-11.
	now := time.Date(2026, 10, 8, 9, 0, 0, 0, time.UTC)
	got := TrustedDateBlock(now)
	for _, want := range []string{"2026-10-08", "Thursday", "2026-10-05", "2026-10-11", `trust="host"`} {
		if !strings.Contains(got, want) {
			t.Errorf("date block missing %q: %s", want, got)
		}
	}
}

func TestStripTags(t *testing.T) {
	in := `<current_date trust="host">Today is X</current_date>`
	if got := StripTags(in); got != "Today is X" {
		t.Errorf("StripTags = %q", got)
	}
}

func TestShortText(t *testing.T) {
	if got := ShortText("abcdef", 3); got != "abc …" {
		t.Errorf("ShortText = %q", got)
	}
	if got := ShortText("ab", 3); got != "ab" {
		t.Errorf("ShortText short = %q", got)
	}
}

// Offline ChatAnswer (no live gateway) must still answer the date from the trusted
// block — the bug the chat upgrades fixed, now a unit test instead of a manual
// browser click. A bogus Dial makes Up() false deterministically.
func TestChatAnswerOfflineUsesTrustedDate(t *testing.T) {
	c := New("http://127.0.0.1:1/unused", "127.0.0.1:1") // unreachable
	db := TrustedDateBlock(time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC))
	ans := c.ChatAnswer(ChatParams{}, "sys", db, "", "", "what's today?", false)
	if !strings.Contains(ans, "2026-10-08") {
		t.Errorf("offline answer should carry the trusted date, got: %s", ans)
	}
	if !strings.Contains(ans, "No matching emails yet") {
		t.Errorf("offline answer with no corpus should say so, got: %s", ans)
	}
}
