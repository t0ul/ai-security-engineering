package main

import (
	"context"
	"testing"
)

// TestModelControllerLifecycle locks the C3 ownership semantics: a fresh controller is
// not running, Stop is an idempotent no-op, and Start fails cleanly (leaving nothing
// running) when no models are available — so start/stop can never orphan or double-start.
// The happy path needs real GGUFs + llama bins and is covered by a live boot.
func TestModelControllerLifecycle(t *testing.T) {
	ctl := newModelController(context.Background(), t.TempDir(), "127.0.0.1:4999", nil)

	if ctl.ModelsRunning() {
		t.Error("a fresh controller must not be running")
	}
	if err := ctl.StopModels(); err != nil {
		t.Errorf("stop on a stopped controller must be a no-op, got %v", err)
	}
	// No catalog / no assets → Available fails → Start errors and stays stopped.
	if err := ctl.StartModels(); err == nil {
		t.Error("start with no available models must return an error")
	}
	if ctl.ModelsRunning() {
		t.Error("a failed start must not leave the controller in a running state")
	}
}
