// Command main is the runnable demo for lesson 06.3, circuit breakers and runaway
// loops.
//
// Run it with:
//
//	go run .
//
// An agent loop that can cycle without reaching a finish is a denial-of-service
// waiting to happen: each step is a model call that costs time and money, and a
// cycle with no cap spins forever. This demo runs a two-node cycle with a naive
// loop that has no limit, then through the agent's real loop engine, whose hard
// step cap trips a circuit breaker and stops it. Deterministic, no model.
//
// Reader safety: the naive loop here is bounded by a ceiling so the demo cannot
// actually hang your machine. In a real agent each iteration is a paid model call,
// so "unbounded" is the whole danger; we show that it never stops on its own, then
// stop it for real with the cap.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/t0ul/gonductor"
)

// safetyCeiling bounds the naive loop so this demo cannot hang. It stands in for
// "forever": reaching it means the loop never terminated on its own.
const safetyCeiling = 100000

// cap is the engine's hard step limit for the defended run.
const cap = 8

func main() {
	ran := naiveSpinsForever()
	tripped := breakerStops()

	fmt.Println()
	if ran && tripped {
		fmt.Println("attack blocked (the receipt): the naive loop never stops itself; the breaker stops it at the cap")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- before: a loop with no limit ----

// naiveLoop walks the same two-step cycle the agent would (plan, act, plan, act),
// with no condition that ever ends it. It stops only at the safety ceiling, which
// is this demo refusing to actually run forever, not the loop deciding to.
func naiveLoop() int {
	steps := 0
	node := "plan"
	for steps < safetyCeiling {
		steps++
		if node == "plan" {
			node = "act"
		} else {
			node = "plan"
		}
		// There is no `if done { break }` here, because nothing makes it done.
	}
	return steps
}

func naiveSpinsForever() bool {
	fmt.Println("== before: a loop with no cap ==")
	steps := naiveLoop()
	hitCeiling := steps >= safetyCeiling
	fmt.Printf("  the loop ran %d steps and still had not finished (stopped only by the demo's ceiling): %v\n", steps, hitCeiling)
	fmt.Println("  in the real agent each step is a paid model call, so this is denial of wallet")
	return hitCeiling // it never terminates on its own: the attack lands
}

// ---- after: the engine's hard step cap ----

// breakerStops builds the same never-ending cycle on the agent's real loop engine,
// which enforces a hard maximum number of steps and fails the run when it is hit.
func breakerStops() bool {
	fmt.Println("\n== after: the loop engine's circuit breaker ==")

	g := gonductor.New[int]()
	g.AddNode("plan", func(_ *gonductor.Context, s int) (int, error) { return s + 1, nil })
	g.AddNode("act", func(_ *gonductor.Context, s int) (int, error) { return s + 1, nil })
	g.AddEdge("plan", "act")
	g.AddEdge("act", "plan") // the cycle: it never routes to END
	g.SetEntry("plan")
	g.MaxSteps(cap)

	compiled, err := g.Compile()
	if err != nil {
		panic(err)
	}
	final, runErr := compiled.Invoke(&gonductor.Context{Context: context.Background()}, 0)

	tripped := errors.Is(runErr, gonductor.ErrCircuitBreaker)
	fmt.Printf("  the same cycle, run with a hard cap of %d steps:\n", cap)
	fmt.Printf("  it stopped after %d steps with: %v\n", final, runErr)
	fmt.Printf("  circuit breaker tripped: %v\n", tripped)
	return tripped && final <= cap
}
