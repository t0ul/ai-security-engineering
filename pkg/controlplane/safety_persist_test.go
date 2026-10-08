package controlplane

import "testing"

// TestSafetyPersistAndRehydrate: Set fires OnSet with the new level (for persistence)
// and Rehydrate restores a level without firing OnSet (a restore, not a new action).
func TestSafetyPersistAndRehydrate(t *testing.T) {
	var persisted KillLevel = -1
	s := NewSafety(nil)
	s.OnSet = func(l KillLevel) { persisted = l }

	s.Set("op", LevelHalt)
	if persisted != LevelHalt {
		t.Fatalf("Set should persist the level via OnSet, got %v", persisted)
	}
	if s.Level() != LevelHalt {
		t.Fatalf("level should be halt, got %v", s.Level())
	}

	// Simulate a fresh process that rehydrates the persisted level.
	persisted = -1
	s2 := NewSafety(nil)
	s2.OnSet = func(l KillLevel) { persisted = l }
	s2.Rehydrate(LevelHalt)
	if s2.Level() != LevelHalt {
		t.Fatalf("rehydrate should restore halt, got %v", s2.Level())
	}
	if persisted != -1 {
		t.Fatalf("rehydrate must NOT fire OnSet (it is a restore, not a new action)")
	}
}
