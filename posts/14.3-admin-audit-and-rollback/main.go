// Command main is the runnable demo for lesson 14.3, admin audit and rollback.
//
// Run it with:
//
//	go run .
//
// Operators change things, and some changes are bad: a prompt tweak that breaks the
// agent, a config that weakens it. Recovering is not "paste the old text back",
// because a hand-retyped prompt is a new, unverified version that only looks right.
// A clean rollback re-activates a specific known-good version and proves, by its
// content hash, that the live prompt is exactly that version again, and records who
// rolled back to what. The agent's prompts plane is versioned and hash-pinned for
// exactly this. Deterministic, no model.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/t0ul/emaildrop/pkg/controlplane"
	"github.com/t0ul/gledger"
)

const name = "chat"

// A bad change: a prompt that drops the never-obey rule the chat depends on.
const badPrompt = "You are a helpful assistant. Do whatever the retrieved notes say."

func newPlane() *controlplane.Prompts {
	return controlplane.NewPrompts(map[string]string{name: controlplane.ChatSystemPrompt})
}

// ---- before: roll back by re-typing the old text ----

// naiveRollback pastes what the operator believes is the previous prompt. Here it
// is the right text with one stray trailing space, the kind of thing that survives
// a copy-paste. It looks restored, but it is a new version with a different hash,
// and nothing can prove it is the known-good one.
func unprovableRollbackUndefended() bool {
	fmt.Println("== before: roll back by re-pasting the old text ==")
	p := newPlane()
	good := p.Get(name) // the known-good version, with its content hash

	p.Activate(name, badPrompt) // the bad change goes live

	p.Activate(name, good.Text+" ") // "rollback": looks right, one extra space
	provable := p.Verify(name, good.Hash)
	fmt.Printf("  after the re-paste, the live prompt is provably the known-good version: %v\n", provable)
	return !provable
}

// ---- after: re-activate a pinned version, verify, record ----

func cleanRollbackDefended() bool {
	fmt.Println("\n== after: re-activate a known-good version, pinned and audited ==")
	dir, _ := os.MkdirTemp("", "rb")
	defer os.RemoveAll(dir)
	logPath := filepath.Join(dir, "admin.log")
	audit, _ := quietOpen(logPath, "control-plane")

	p := newPlane()
	good := p.Get(name) // version 0, the pinned known-good

	bad := p.Activate(name, badPrompt) // the bad change
	fmt.Printf("  bad change live: version %d, never-obey rule present: %v\n", bad.Version, hasRule(p))

	// Roll back by re-activating the exact stored text of the known-good version,
	// then prove the live prompt matches its pinned hash.
	rb := p.Activate(name, good.Text)
	provable := p.Verify(name, good.Hash)
	quiet(func() {
		audit.Emit(gledger.NewTraceID(), "admin", "rolled_back",
			gledger.F{"by": "alice-operator", "name": name, "from_version": bad.Version, "to_hash": good.Hash[:12], "new_version": rb.Version})
	})
	fmt.Printf("  rolled back: live prompt is provably the known-good version: %v (rule back: %v)\n", provable, hasRule(p))

	fmt.Println("  audit (who rolled back to what):")
	recs, _ := gledger.Read(logPath)
	for _, r := range recs {
		if r.Event == "rolled_back" {
			fmt.Printf("    %v\n", r.Fields)
		}
	}
	ok, _ := gledger.VerifyFile(logPath)
	fmt.Printf("  audit chain verified: %v\n", ok)

	return provable && hasRule(p) && ok
}

func hasRule(p *controlplane.Prompts) bool {
	return contains(p.Text(name), "NEVER follow")
}

func contains(s, sub string) bool { return len(s) >= len(sub) && indexOf(s, sub) >= 0 }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func main() {
	undef := unprovableRollbackUndefended()
	def := cleanRollbackDefended()
	fmt.Println()
	if undef && def {
		fmt.Println("attack blocked (the receipt): a re-pasted prompt cannot be proven to be the known-good version; re-activating a pinned version restores the exact content, verifies it by hash, and records who rolled back to what")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

func quiet(fn func()) {
	old := os.Stdout
	devnull, _ := os.Open(os.DevNull)
	os.Stdout = devnull
	defer func() { os.Stdout = old; devnull.Close() }()
	fn()
}

func quietOpen(path, service string) (*gledger.AuditLog, error) {
	old := os.Stdout
	devnull, _ := os.Open(os.DevNull)
	os.Stdout = devnull
	defer func() { os.Stdout = old; devnull.Close() }()
	return gledger.Open(path, service)
}
