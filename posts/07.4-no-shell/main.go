// Command main is the runnable demo for lesson 07.4, no shell.
//
// Run it with:
//
//	go run .
//
// A tool is called with arguments a model produced from an email. If those
// arguments are pasted into a shell string, the shell is a programming language the
// attacker gets to write in: metacharacters run commands, and ../ escapes the
// allowed folder. This demo runs a grep tool the naive way, as a shell string, and
// watches an argument inject a command and reach a file it should not, then through
// the agent's real validator, which passes arguments as an array against a schema
// so there is no shell to be clever with. Deterministic, no model.
//
// Reader safety: the injected command only writes a marker; it is a harmless stand-in
// for the destructive command a real attack would use.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/t0ul/emaildrop/pkg/argcheck"
)

// schema declares the grep tool's two arguments and their kinds.
var schema = argcheck.Schema{"pattern": argcheck.String, "file": argcheck.String}

var (
	baseDir string // the one folder the tool may read from
	marker  string // the file an injected command would create
)

func main() {
	dir, err := os.MkdirTemp("", "lesson-07.4-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	baseDir = filepath.Join(dir, "base")
	os.MkdirAll(baseDir, 0o755)
	os.WriteFile(filepath.Join(baseDir, "notes.txt"), []byte("the word root appears here\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("SECRET-OUTSIDE-THE-BASE\n"), 0o644) // outside base
	marker = filepath.Join(dir, "injected_marker.txt")

	injected := argInjectionLands()
	escaped := traversalLands()
	blocked := validatorBlocksBoth()

	fmt.Println()
	if injected && escaped && blocked {
		fmt.Println("attack blocked (the receipt): a shell string lets arguments run commands and escape; the validator does not")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- before: arguments pasted into a shell string ----

// naiveGrep builds a shell command string from the arguments and runs it. The two
// string concatenations are the whole vulnerability.
func naiveGrep(pattern, file string) string {
	cmd := "grep " + pattern + " " + filepath.Join(baseDir, file)
	out, _ := exec.Command("sh", "-c", cmd).CombinedOutput()
	return string(out)
}

func argInjectionLands() bool {
	fmt.Println("== before: arguments in a shell string ==")
	os.Remove(marker)
	// A pattern that closes the grep and runs its own command.
	pattern := "x; echo INJECTED-COMMAND-RAN > " + marker + " #"
	naiveGrep(pattern, "notes.txt")
	_, err := os.Stat(marker)
	ran := err == nil
	fmt.Printf("  pattern = %q\n  an injected command ran (marker file created): %v\n", pattern, ran)
	return ran
}

func traversalLands() bool {
	out := naiveGrep("SECRET", "../secret.txt")
	escaped := strings.Contains(out, "SECRET-OUTSIDE-THE-BASE")
	fmt.Printf("  file = \"../secret.txt\" read a file outside the base folder: %v\n", escaped)
	return escaped
}

// ---- after: arguments validated and passed as an array ----

// safeGrep validates the arguments against the schema and confines the path, then
// runs grep with an argument ARRAY. There is no shell, so metacharacters are just
// characters and there is nothing to inject into.
func safeGrep(pattern, file string) (string, error) {
	if _, err := argcheck.Validate(schema, map[string]any{"pattern": pattern, "file": file}); err != nil {
		return "", err
	}
	path, err := argcheck.ConfinePath(baseDir, file)
	if err != nil {
		return "", err
	}
	out, _ := exec.Command("grep", pattern, path).CombinedOutput() // argv array, no shell
	return string(out), nil
}

func validatorBlocksBoth() bool {
	fmt.Println("\n== after: the validator + an argument array ==")
	os.Remove(marker)

	_, errInj := safeGrep("x; echo INJECTED-COMMAND-RAN > "+marker+" #", "notes.txt")
	_, markerErr := os.Stat(marker)
	noMarker := markerErr != nil
	fmt.Printf("  the injecting pattern: %s (no command ran: %v)\n", short(errInj), noMarker)

	_, errTrav := safeGrep("SECRET", "../secret.txt")
	fmt.Printf("  the traversing file:   %s\n", short(errTrav))

	// And a clean call still works, as an argv array with no shell.
	okOut, errOk := safeGrep("root", "notes.txt")
	fmt.Printf("  a clean call:          %s", result(okOut, errOk))

	return errInj != nil && noMarker && errTrav != nil && errOk == nil && strings.Contains(okOut, "root")
}

// ---- helpers ----

func short(err error) string {
	if err == nil {
		return "ALLOWED (should not happen)"
	}
	s := err.Error()
	return "refused: " + strings.TrimPrefix(s, "argcheck: ")
}

func result(out string, err error) string {
	if err != nil {
		return "error: " + err.Error() + "\n"
	}
	return "found: " + strings.TrimSpace(out) + "\n"
}
