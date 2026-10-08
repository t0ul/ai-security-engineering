// Package durable is the agent's durable-execution + exactly-once spine. It is an
// append-only, hash-chained log of completed steps (model turns and side-effecting
// tool calls), keyed so that:
//
//   - Durable execution: re-running a session after a crash or rate-limit replays
//     each already-completed step's cached result instead of re-executing it, so
//     the agent resumes from the last good step rather than restarting.
//   - Exactly-once side effects: a retried or duplicated action keyed to the same
//     idempotency key returns the first result and never fires the side effect
//     twice (no double-send / double-refund).
//
// The log is hash-chained like the audit ledger, which makes resume *verifiable*:
// before trusting a cached step on resume, Verify recomputes the chain and fails
// closed if any record was edited — you cannot rewind the agent into a forged
// checkpoint. Stdlib only; file-backing is optional (empty path = in-memory).
package durable

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"sync"
)

// ErrTampered means the on-disk chain does not recompute — a step was edited,
// reordered, or removed. Resume fails closed.
var ErrTampered = errors.New("durable: step log hash-chain does not verify (tampered checkpoint)")

// Step is one recorded, completed step. Hash chains it to PrevHash.
type Step struct {
	Session  string `json:"session"`
	Seq      int    `json:"seq"`
	Kind     string `json:"kind"` // "model" | "tool"
	Key      string `json:"key"`  // idempotency key (tool) or step name (model)
	Output   string `json:"output"`
	PrevHash string `json:"prev"`
	Hash     string `json:"hash"`
}

// stepHash is the content pin: a completed step's fields over the previous hash.
func stepHash(s Step) string {
	h := sha256.New()
	h.Write([]byte(s.Session + "\x1f" + strconv.Itoa(s.Seq) + "\x1f" + s.Kind + "\x1f" + s.Key + "\x1f" + s.Output + "\x1f" + s.PrevHash))
	return hex.EncodeToString(h.Sum(nil))
}

func indexKey(session, key string) string { return session + "\x00" + key }

// Log is the durable step store.
type Log struct {
	mu       sync.Mutex
	path     string
	steps    []Step
	byKey    map[string]Step
	lastHash string
}

// Open loads the log at path (empty = in-memory only) and verifies its chain. A
// tampered on-disk log fails closed with ErrTampered rather than resuming into a
// forged state.
func Open(path string) (*Log, error) {
	l := &Log{path: path, byKey: map[string]Step{}}
	if path == "" {
		return l, nil
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return l, nil // fresh log
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var s Step
		if err := json.Unmarshal(line, &s); err != nil {
			return nil, err
		}
		l.steps = append(l.steps, s)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if err := l.verifyLocked(); err != nil {
		return nil, err
	}
	for _, s := range l.steps {
		l.byKey[indexKey(s.Session, s.Key)] = s
		l.lastHash = s.Hash
	}
	return l, nil
}

// verifyLocked recomputes the whole chain and reports the first divergence.
func (l *Log) verifyLocked() error {
	prev := ""
	for i, s := range l.steps {
		want := s
		want.PrevHash = prev
		want.Hash = stepHash(want)
		if s.PrevHash != prev || s.Hash != want.Hash || s.Seq != i {
			return ErrTampered
		}
		prev = s.Hash
	}
	return nil
}

// Verify re-checks the in-memory chain (call on resume before trusting a cached
// step, or any time you want to detect tampering).
func (l *Log) Verify() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.verifyLocked()
}

// Lookup returns the recorded step for session+key, if the step already completed.
func (l *Log) Lookup(session, key string) (Step, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	s, ok := l.byKey[indexKey(session, key)]
	return s, ok
}

// Do runs fn exactly once for (session, key). On the first call it executes fn and
// records a successful result; a later call with the same key returns the cached
// output with replayed=true and never calls fn again — this is both the
// durable-resume replay and the exactly-once side-effect guard. A failing fn is
// NOT recorded, so a transient failure can be safely retried; only success is
// durable (exactly-once-on-success). kind is "model" or "tool".
func (l *Log) Do(session, kind, key string, fn func() (string, error)) (output string, replayed bool, err error) {
	if s, ok := l.Lookup(session, key); ok {
		return s.Output, true, nil
	}
	out, ferr := fn()
	if ferr != nil {
		return "", false, ferr // not recorded: retryable
	}
	if _, rerr := l.record(session, kind, key, out); rerr != nil {
		return out, false, rerr
	}
	return out, false, nil
}

// record appends a completed step and chains it. Serialized under the lock so the
// Seq/PrevHash are consistent.
func (l *Log) record(session, kind, key, output string) (Step, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := Step{Session: session, Seq: len(l.steps), Kind: kind, Key: key, Output: output, PrevHash: l.lastHash}
	s.Hash = stepHash(s)
	if l.path != "" {
		if err := l.appendLine(s); err != nil {
			return Step{}, err
		}
	}
	l.steps = append(l.steps, s)
	l.byKey[indexKey(session, key)] = s
	l.lastHash = s.Hash
	return s, nil
}

func (l *Log) appendLine(s Step) error {
	f, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return f.Sync()
}
