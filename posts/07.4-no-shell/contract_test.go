package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Deterministic, no model. Arguments in a shell string let an injected command run
// and a ../ path escape; the validator refuses both and a clean call still works.

func setup(t *testing.T) {
	dir := t.TempDir()
	baseDir = filepath.Join(dir, "base")
	os.MkdirAll(baseDir, 0o755)
	os.WriteFile(filepath.Join(baseDir, "notes.txt"), []byte("the word root appears here\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("SECRET-OUTSIDE-THE-BASE\n"), 0o644)
	marker = filepath.Join(dir, "injected_marker.txt")
}

func TestShellStringLetsAttacksLand(t *testing.T) {
	setup(t)
	if !argInjectionLands() {
		t.Fatal("a shell string should let the injected command run; otherwise there is nothing to fix")
	}
	if !traversalLands() {
		t.Fatal("a shell string should let ../ read outside the base folder")
	}
}

func TestValidatorBlocksBoth(t *testing.T) {
	setup(t)
	if !validatorBlocksBoth() {
		t.Fatal("the validator must refuse the injection and the traversal, and still run a clean call")
	}
}

func TestMetacharactersRejected(t *testing.T) {
	setup(t)
	for _, bad := range []string{"a;b", "$(x)", "`y`", "a&&b", "a|b"} {
		if _, err := safeGrep(bad, "notes.txt"); err == nil {
			t.Fatalf("expected %q to be refused", bad)
		}
	}
	if !strings.Contains(mustGrep(t, "root"), "root") {
		t.Fatal("a clean pattern must still match")
	}
}

func mustGrep(t *testing.T, p string) string {
	out, err := safeGrep(p, "notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	return out
}
