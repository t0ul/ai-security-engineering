// Package captoken issues short-lived, scoped, per-call capability tokens so
// there are no ambient credentials for a compromised request to steal (M16). A
// token names exactly one scope and expires quickly; a stolen, expired, or
// wrong-scope token fails verification. HMAC-signed, stdlib only.
package captoken

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

var (
	ErrBadToken = errors.New("captoken: malformed or forged token")
	ErrExpired  = errors.New("captoken: token expired")
	ErrScope    = errors.New("captoken: token not valid for this scope")
)

// Minter issues and verifies tokens under one secret key.
type Minter struct {
	key []byte
	// Now overrides the clock for tests.
	Now func() time.Time
}

// NewMinter returns a minter keyed by key.
func NewMinter(key []byte) *Minter { return &Minter{key: key} }

func (m *Minter) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m *Minter) sign(payload string) string {
	h := hmac.New(sha256.New, m.key)
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// Mint returns a token authorizing scope until now+ttl.
func (m *Minter) Mint(scope string, ttl time.Duration) string {
	// Hex-encode the scope so it can never collide with the field separators.
	payload := hex.EncodeToString([]byte(scope)) + "|" + strconv.FormatInt(m.now().Add(ttl).Unix(), 10) + "|" + nonce()
	return payload + "." + m.sign(payload)
}

// Verify checks the token's signature, expiry, and that it carries requiredScope.
func (m *Minter) Verify(token, requiredScope string) error {
	payload, sig, ok := strings.Cut(token, ".")
	if !ok {
		return ErrBadToken
	}
	if !hmac.Equal([]byte(m.sign(payload)), []byte(sig)) {
		return ErrBadToken
	}
	parts := strings.SplitN(payload, "|", 3)
	if len(parts) != 3 {
		return ErrBadToken
	}
	exp, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return ErrBadToken
	}
	if m.now().Unix() > exp {
		return ErrExpired
	}
	if parts[0] != hex.EncodeToString([]byte(requiredScope)) {
		return ErrScope
	}
	return nil
}

func nonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
