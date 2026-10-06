// Command scorecard is the M12 full-chain security gate: it runs every red-team
// technique against the undefended baseline and the real control, printing the
// attack-success-rate before and after each defense. It exits non-zero if any
// defended ASR is above zero, so it doubles as a CI security-regression gate —
// a real incident becomes a permanent test that cannot silently recur.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/t0ul/ai-security-engineering/redteam"
	"github.com/t0ul/gorauder"
)

func main() {
	ctx := context.Background()
	asr := func(target gorauder.Target, seeds []gorauder.Seed) float64 {
		return gorauder.NewRunner(target, gorauder.WithScorer(redteam.Scorer())).Run(ctx, seeds).ASR()
	}

	fmt.Println("== security scorecard (ASR before -> after) ==")
	fmt.Printf("%-22s %-14s %8s %8s\n", "technique", "control", "before", "after")

	failed := 0
	total := 0
	for _, c := range redteam.Cases() {
		before := asr(c.Undefended, c.Seeds) * 100
		after := asr(c.Defended, c.Seeds) * 100
		flag := "ok"
		if after > 0 {
			flag = "FAIL"
			failed++
		}
		total++
		fmt.Printf("%-22s %-14s %7.0f%% %7.0f%%  %s\n", c.Name, c.Technique, before, after, flag)
	}

	fmt.Printf("\n%d technique(s); %d regressed\n", total, failed)
	if failed > 0 {
		os.Exit(1)
	}
	fmt.Println("all defenses hold: ASR 0% across the chain.")
}
