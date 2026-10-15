// Command main is the runnable demo for lesson 12.1, the one-command scorecard.
//
// Run it with:
//
//	go run .
//
// The whole book is a pile of before/after tests. This post turns that pile into
// one command that scores every control and reds the build if any regresses. But
// a scorecard is only worth running if each row is wired: the attack must really
// land with the control removed, and the control must really be on the path. This
// demo runs three rows through the agent's real Attack-Driven Design gate, two of
// them the false-green rows this book keeps warning about, and shows the gate
// catching them. Deterministic, no model.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/t0ul/ADD"
	"github.com/t0ul/gorauder"
)

func main() {
	rows := []add.Technique{wiredRow(), stubbedRow(), notWiredRow()}

	fmt.Println("== before: a scorecard that only checks the defended arm ==")
	for _, r := range rows {
		o, _ := report(r)
		fmt.Printf("  %-26s defended ASR %.0f%% -> looks GREEN\n", o.Name, o.DefendedASR*100)
	}
	fmt.Println("  (all three look identical; two of them are false assurance)")

	fmt.Println("\n== after: the ADD gate's invariants ==")
	allPassWhenExpected := true
	for _, r := range rows {
		o, v := report(r)
		if len(v) == 0 {
			fmt.Printf("  %-26s PASS: undefended %.0f%% -> defended %.0f%% -> wired %.0f%%\n",
				o.Name, o.UndefendedASR*100, o.DefendedASR*100, o.WiredASR*100)
		} else {
			fmt.Printf("  %-26s FLAGGED: %s\n", o.Name, strings.Join(v, "; "))
		}
	}

	// The gate must: pass the wired row, flag the stubbed row, flag the not-wired row.
	_, wiredV := report(wiredRow())
	_, stubV := report(stubbedRow())
	_, nwV := report(notWiredRow())
	ok := len(wiredV) == 0 && hasAny(stubV, "NOT PROVEN") && hasAny(nwV, "NOT WIRED")
	if !ok {
		allPassWhenExpected = false
	}

	fmt.Println()
	if allPassWhenExpected {
		fmt.Println("attack blocked (the receipt): the gate passes only the wired row and flags both false-green rows, so a scorecard that is all green actually means every control is attacked and on the path")
	} else {
		fmt.Println("the gate did not distinguish wired from stubbed")
		os.Exit(1)
	}
}

// ---- one attack, one control ----

const marker = "PWNED"

// seeds is one attack: a prompt carrying a success marker. If a target's response
// still contains the marker, the attack beat it.
var seeds = []gorauder.Seed{
	{ID: "inj-01", Technique: "marker-echo", Prompt: "summarize this: " + marker, Marker: marker},
}

// strip is the real control: it removes the marker. Simple on purpose; the lesson
// is about how the row is wired, not how clever the control is.
func strip(s string) string { return strings.ReplaceAll(s, marker, "") }

// ---- the targets a row can be built from ----

// echo is the control REMOVED: the prompt passes through untouched, so the attack
// lands. This is the counterfactual that proves the attack is real.
func echo(_ context.Context, prompt string) (string, error) { return prompt, nil }

// defended is the real control applied directly.
func defended(_ context.Context, prompt string) (string, error) { return strip(prompt), nil }

// wiredPipeline is the real end-to-end path with the control ON it.
func wiredPipeline(_ context.Context, prompt string) (string, error) {
	cleaned := strip(prompt) // the control sits in the pipeline
	return "processed: " + cleaned, nil
}

// leakyPipeline is the real path with the control defined but NOT called: the
// classic bug where a sanitizer exists in the codebase but nothing runs it.
func leakyPipeline(_ context.Context, prompt string) (string, error) {
	return "processed: " + prompt, nil // forgot to strip
}

// stub is a fake undefended arm: it reports safe without ever exposing the attack.
// A row built on this is green and proves nothing.
func stub(_ context.Context, _ string) (string, error) { return "[safe]", nil }

// ---- three rows ----

func wiredRow() add.Technique {
	return add.Technique{
		Name: "marker-strip (wired)", Risk: "LLM01", Seeds: seeds,
		Undefended: gorauder.TargetFunc(echo),
		Defended:   gorauder.TargetFunc(defended),
		Wired:      gorauder.TargetFunc(wiredPipeline),
	}
}

func stubbedRow() add.Technique {
	return add.Technique{
		Name: "marker-strip (stubbed)", Risk: "LLM01", Seeds: seeds,
		Undefended: gorauder.TargetFunc(stub), // never attacks
		Defended:   gorauder.TargetFunc(stub),
	}
}

func notWiredRow() add.Technique {
	return add.Technique{
		Name: "marker-strip (not wired)", Risk: "LLM01", Seeds: seeds,
		Undefended: gorauder.TargetFunc(echo),
		Defended:   gorauder.TargetFunc(defended),      // the unit control works
		Wired:      gorauder.TargetFunc(leakyPipeline), // but the real path skips it
	}
}

func report(tech add.Technique) (add.Outcome, []string) { return add.Evaluate(tech) }

// ---- helpers ----

func hasAny(violations []string, sub string) bool {
	for _, v := range violations {
		if strings.Contains(v, sub) {
			return true
		}
	}
	return false
}
