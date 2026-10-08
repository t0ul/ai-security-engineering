// Package a2a secures agent-to-agent hand-offs (OWASP ASI07, insecure
// inter-agent communication). In a multi-agent loop one agent's output is
// another's input; if the executor trusts any message claiming to be "from the
// planner", an injected COMMAND can spoof the control flow. a2a gives each agent
// an identity and a key, signs every message (HMAC-SHA256 over sender, nonce,
// and body), and has the receiver verify the signature, the sender's identity,
// and that the nonce has not been replayed before acting.
package a2a

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrUnknownAgent = errors.New("a2a: message from an unknown agent")
	ErrBadSignature = errors.New("a2a: signature does not verify (forged or tampered)")
	ErrReplay       = errors.New("a2a: nonce already seen (replay)")
)

// Message is a signed inter-agent hand-off.
type Message struct {
	From  string `json:"from"`
	Body  string `json:"body"`
	Nonce string `json:"nonce"`
	Sig   string `json:"sig"` // hex HMAC-SHA256(key, from|nonce|body)
}

// Signer holds one agent's identity and key.
type Signer struct {
	AgentID string
	key     []byte
}

// NewSigner returns a signer for agentID with the given secret key.
func NewSigner(agentID string, key []byte) *Signer { return &Signer{AgentID: agentID, key: key} }

func mac(key []byte, from, nonce, body string) string {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(from))
	h.Write([]byte{0x1f})
	h.Write([]byte(nonce))
	h.Write([]byte{0x1f})
	h.Write([]byte(body))
	return hex.EncodeToString(h.Sum(nil))
}

// Sign produces a signed Message carrying body, with a fresh random nonce.
func (s *Signer) Sign(body string) Message {
	nb := make([]byte, 16)
	_, _ = rand.Read(nb)
	nonce := hex.EncodeToString(nb)
	return Message{From: s.AgentID, Body: body, Nonce: nonce, Sig: mac(s.key, s.AgentID, nonce, body)}
}

// maxSeenNonces bounds the replay-guard memory. When the live set fills, it
// rotates to a previous generation (both are still checked), so memory stays
// bounded at ~2× this on a long-running verifier.
const maxSeenNonces = 100_000

// Verifier checks messages against the registered agent keys and rejects replays.
type Verifier struct {
	mu   sync.Mutex
	keys map[string][]byte
	seen map[string]bool
	old  map[string]bool
}

// NewVerifier builds a verifier. Register agents with Trust.
func NewVerifier() *Verifier {
	return &Verifier{keys: map[string][]byte{}, seen: map[string]bool{}, old: map[string]bool{}}
}

// Trust registers agentID's key so its messages verify.
func (v *Verifier) Trust(agentID string, key []byte) *Verifier {
	v.mu.Lock()
	v.keys[agentID] = key
	v.mu.Unlock()
	return v
}

// Verify authenticates m: the sender must be trusted, the signature must match
// (constant-time), and the nonce must be unseen. On success the nonce is
// recorded so it cannot be replayed.
func (v *Verifier) Verify(m Message) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	key, ok := v.keys[m.From]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownAgent, m.From)
	}
	want := mac(key, m.From, m.Nonce, m.Body)
	if !hmac.Equal([]byte(want), []byte(m.Sig)) {
		return ErrBadSignature
	}
	replayKey := m.From + ":" + m.Nonce
	if v.seen[replayKey] || v.old[replayKey] {
		return ErrReplay
	}
	if len(v.seen) >= maxSeenNonces {
		v.old = v.seen // rotate; keep one previous generation
		v.seen = map[string]bool{}
	}
	v.seen[replayKey] = true
	return nil
}
