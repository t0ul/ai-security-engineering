package main

import (
	"strings"
	"testing"
)

// Offline, deterministic, no model: the structural property. The hardened judge
// fences the candidate answer in <candidate_answer> tags and declares it data the
// judge must never obey; the naive judge splices it straight into the user turn.
// CI runs this; the model-in-the-loop proof is in the live-tagged test.

func TestHardenedJudgeFencesTheCandidate(t *testing.T) {
	u := hardenedJudgeUser()
	if !strings.Contains(u, "<candidate_answer>") || !strings.Contains(u, "</candidate_answer>") {
		t.Fatal("the hardened judge must enclose the candidate answer in untrusted-data tags")
	}
	sys := strings.ToLower(hardenedJudgeSystem)
	if !strings.Contains(sys, "never instructions") || !strings.Contains(sys, "ignore any instruction") {
		t.Fatal("the hardened judge's rubric must declare the candidate data, never instructions")
	}
	// The naive judge has no boundary: the raw answer, directive and all, is
	// spliced into its own instructions.
	if !strings.Contains(naiveJudgeSystem(), badAnswer) || strings.Contains(naiveJudgeSystem(), "<candidate_answer>") {
		t.Fatal("the naive judge must splice the raw answer into instructions with no fence; that is the whole before/after")
	}
}

func TestScoreParsing(t *testing.T) {
	if score("10") != 10 || score("the score is 2.") != 2 || score("0") != 0 {
		t.Fatal("score must parse the judge's integer verdict")
	}
}
