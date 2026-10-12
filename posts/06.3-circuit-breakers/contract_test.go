package main

import (
	"context"
	"errors"
	"testing"

	"github.com/t0ul/gonductor"
)

// Deterministic, no model. The naive loop never terminates on its own (it reaches
// the demo's safety ceiling); the engine's hard cap trips the circuit breaker and
// stops the same cycle.

func TestNaiveLoopNeverTerminates(t *testing.T) {
	if !naiveSpinsForever() {
		t.Fatal("the uncapped loop should run to the ceiling without finishing; otherwise there is nothing to fix")
	}
}

func TestCircuitBreakerTrips(t *testing.T) {
	if !breakerStops() {
		t.Fatal("the engine's step cap must trip the circuit breaker on an endless cycle")
	}
}

func TestBreakerErrorIsSentinel(t *testing.T) {
	g := gonductor.New[int]()
	g.AddNode("a", func(_ *gonductor.Context, s int) (int, error) { return s + 1, nil })
	g.AddEdge("a", "a")
	g.SetEntry("a")
	g.MaxSteps(3)
	c, err := g.Compile()
	if err != nil {
		t.Fatal(err)
	}
	if _, runErr := c.Invoke(&gonductor.Context{Context: context.Background()}, 0); !errors.Is(runErr, gonductor.ErrCircuitBreaker) {
		t.Fatalf("expected ErrCircuitBreaker, got %v", runErr)
	}
}
