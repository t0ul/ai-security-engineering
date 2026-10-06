// Package memory is the M20 agent-memory lifecycle: persistent memory is a
// poisoning sink and a PII hoard, so entries are per-subject scoped (no
// cross-scope recall), carry a TTL (expired entries are never recalled),
// support right-to-erasure (DSAR, M19), and — because recalled memory is
// untrusted content — are injection-neutralized when assembled for a prompt.
package memory

import (
	"sync"
	"time"

	"github.com/t0ul/ai-security-engineering/agent/guard"
)

// Entry is one memory. Untrusted marks memory derived from attacker-influenced
// content (e.g. a summarized email), which must be sanitized on recall.
type Entry struct {
	Scope     string
	Key       string
	Value     string
	Untrusted bool
	expiresAt time.Time // zero = no expiry
}

// Store is scope-partitioned memory.
type Store struct {
	mu      sync.Mutex
	byScope map[string]map[string]Entry
	// Now overrides the clock for tests.
	Now func() time.Time
}

// New returns an empty store.
func New() *Store { return &Store{byScope: map[string]map[string]Entry{}} }

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Put stores value under (scope,key). ttl<=0 means no expiry.
func (s *Store) Put(scope, key, value string, ttl time.Duration, untrusted bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := Entry{Scope: scope, Key: key, Value: value, Untrusted: untrusted}
	if ttl > 0 {
		e.expiresAt = s.now().Add(ttl)
	}
	if s.byScope[scope] == nil {
		s.byScope[scope] = map[string]Entry{}
	}
	s.byScope[scope][key] = e
}

func (s *Store) live(e Entry) bool {
	return e.expiresAt.IsZero() || s.now().Before(e.expiresAt)
}

// Get returns a live entry from the caller's own scope only.
func (s *Store) Get(scope, key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.byScope[scope][key]
	if !ok || !s.live(e) {
		return "", false
	}
	return e.Value, true
}

// Recall returns all live entries for a scope (recalled memory is untrusted).
func (s *Store) Recall(scope string) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Entry
	for _, e := range s.byScope[scope] {
		if s.live(e) {
			out = append(out, e)
		}
	}
	return out
}

// Erase removes every entry for a scope (DSAR / right to erasure) and returns
// the count removed.
func (s *Store) Erase(scope string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := len(s.byScope[scope])
	delete(s.byScope, scope)
	return n
}

// Assemble renders recalled entries for a prompt, injection-neutralizing the
// untrusted ones so poisoned memory cannot act as instructions.
func Assemble(entries []Entry) string {
	var out string
	for _, e := range entries {
		v := e.Value
		if e.Untrusted {
			v, _ = guard.Sanitize(v)
		}
		out += "<memory key=\"" + e.Key + "\">\n" + v + "\n</memory>\n"
	}
	return out
}
