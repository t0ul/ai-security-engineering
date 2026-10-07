// Package provenance marks what the agent produced (M20 output authenticity /
// C2PA-style content credentials): a detached ed25519 signature over the output
// bytes. A verifier that trusts the signing key can confirm a given output was
// produced by this agent and has not been altered — any tampering fails
// verification. Stdlib crypto only.
package provenance

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
)

var (
	ErrUnknownKey   = errors.New("provenance: signature from an untrusted key")
	ErrBadSignature = errors.New("provenance: signature does not verify (forged or tampered output)")
)

// Mark is a detached content credential over some output.
type Mark struct {
	KeyID string `json:"key_id"`
	Alg   string `json:"alg"` // "ed25519"
	Sig   string `json:"sig"` // hex signature
}

// Signer holds one agent identity's signing key.
type Signer struct {
	keyID string
	priv  ed25519.PrivateKey
}

// NewSigner generates a key pair and returns the signer plus its public key to
// distribute to verifiers.
func NewSigner(keyID string) (*Signer, ed25519.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return &Signer{keyID: keyID, priv: priv}, pub, nil
}

// SignerFromSeed reconstructs a signer from a persisted 32-byte ed25519 seed, so
// an agent keeps a stable identity (and verifiable public key) across restarts.
func SignerFromSeed(keyID string, seed []byte) (*Signer, ed25519.PublicKey, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, nil, fmt.Errorf("provenance: seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	return &Signer{keyID: keyID, priv: priv}, pub, nil
}

// Sign produces a content credential for content.
func (s *Signer) Sign(content []byte) Mark {
	return Mark{KeyID: s.keyID, Alg: "ed25519", Sig: hex.EncodeToString(ed25519.Sign(s.priv, content))}
}

// Verifier checks marks against trusted public keys.
type Verifier struct{ keys map[string]ed25519.PublicKey }

// NewVerifier returns an empty verifier; register keys with Trust.
func NewVerifier() *Verifier { return &Verifier{keys: map[string]ed25519.PublicKey{}} }

// Trust registers a signer's public key by id.
func (v *Verifier) Trust(keyID string, pub ed25519.PublicKey) *Verifier {
	v.keys[keyID] = pub
	return v
}

// Verify confirms content was signed by the mark's trusted key and is unaltered.
func (v *Verifier) Verify(content []byte, m Mark) error {
	pub, ok := v.keys[m.KeyID]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownKey, m.KeyID)
	}
	sig, err := hex.DecodeString(m.Sig)
	if err != nil || !ed25519.Verify(pub, content, sig) {
		return ErrBadSignature
	}
	return nil
}
