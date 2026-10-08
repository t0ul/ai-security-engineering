package controlplane

import (
	"sync"

	"github.com/t0ul/gledger"
)

// KillLevel is the layered kill switch (M18): a single escalating dial, each
// level strictly more restrictive than the last. A binary on/off cannot express
// "let planning continue but stop all side effects"; the levels can.
type KillLevel int

const (
	LevelNone       KillLevel = iota // normal operation
	LevelBlockTools                  // sessions run, but no tool / MCP / sandbox execution (stop side effects now)
	LevelPause                       // refuse new requests (drain in-flight)
	LevelHalt                        // full stop (terminal; pair with drain + snapshot)
)

func (l KillLevel) String() string {
	switch l {
	case LevelBlockTools:
		return "block_tools"
	case LevelPause:
		return "pause"
	case LevelHalt:
		return "halt"
	default:
		return "none"
	}
}

// Safety holds the current kill level and answers the two gates the agent loop
// consults. It satisfies the orchestrator's SafetyGate. Changes are audited.
type Safety struct {
	mu    sync.Mutex
	level KillLevel
	audit *gledger.AuditLog
}

// NewSafety returns a Safety at LevelNone.
func NewSafety(audit *gledger.AuditLog) *Safety { return &Safety{audit: audit} }

// Set changes the kill level, recording who and what.
func (s *Safety) Set(by string, level KillLevel) {
	s.mu.Lock()
	s.level = level
	s.mu.Unlock()
	if s.audit != nil {
		s.audit.Emit(gledger.NewTraceID(), "safety", "set_level", gledger.F{"by": by, "level": level.String()})
	}
}

// Level reports the current kill level.
func (s *Safety) Level() KillLevel {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.level
}

// AllowRequest reports whether a new request may start (false at Pause/Halt).
func (s *Safety) AllowRequest() bool { return s.Level() < LevelPause }

// AllowToolExec reports whether tool/MCP/sandbox execution may run (false at
// BlockTools and above).
func (s *Safety) AllowToolExec() bool { return s.Level() < LevelBlockTools }
