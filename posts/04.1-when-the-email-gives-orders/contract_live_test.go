//go:build live

// Live end-to-end proof: boots nothing, but talks to the local model from post
// 00.2 (planner on 127.0.0.1:11435). Excluded from offline `go test` and from
// GitHub CI, which has no model; the local pre-push gate and `make capture` run it.
// Run with:
//
//	go test -tags live -v ./...
//
// It must fail if the model is down, never skip: a test that stubs the model is
// not testing the thing this lesson is about.
package main

import (
	"context"
	"os"
	"testing"
)

func TestInjectionLandsUndefendedBlockedDefended(t *testing.T) {
	os.Setenv("GATEWAY_URL", planner)
	os.Setenv("EXTRACT_MODE", "llm")
	ctx := context.Background()

	raw, err := undefended(ctx)
	if err != nil {
		t.Fatalf("model unreachable (bring up the lab from post 00.2): %v", err)
	}
	if !marker(raw) {
		t.Fatal("the undefended model should obey the injection; otherwise there is nothing to fix")
	}

	for i := 0; i < 3; i++ {
		ev := defended()
		joined := ""
		for _, e := range ev {
			joined += " " + e.Title + " " + e.Start + " " + e.Location + " " + e.URL
		}
		if marker(joined) {
			t.Fatalf("the injected event survived the real controls on run %d: %v", i, ev)
		}
	}
}
