// Package hitl models an evidence-first, clickjack-resistant human approval.
// People rubber-stamp confident AI (automation bias), and a one-click approval
// dialog can be clickjacked or UI-redressed. An approval here is valid only when
// the operator was shown evidence and echoes back the nonce displayed with THIS
// request — a blind or overlaid click cannot produce it, and an evidence-free
// request cannot be confirmed.
package hitl

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
)

var (
	ErrNoEvidence    = errors.New("hitl: approval requires evidence to review")
	ErrNonceMismatch = errors.New("hitl: nonce mismatch (dialog not the one shown; possible clickjack)")
)

// Request is an approval ask bound to a nonce.
type Request struct {
	Summary  string
	Evidence []string
	nonce    string
}

// NewRequest builds a request with a fresh nonce. Evidence is what the operator
// must see before deciding.
func NewRequest(summary string, evidence ...string) Request {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return Request{Summary: summary, Evidence: evidence, nonce: hex.EncodeToString(b)}
}

// Nonce is the value the UI must display and the operator must echo on confirm.
func (r Request) Nonce() string { return r.nonce }

// Confirm approves only when evidence was present, the echoed nonce matches this
// request, and approve is explicitly true.
func (r Request) Confirm(echoedNonce string, approve bool) (bool, error) {
	if len(r.Evidence) == 0 {
		return false, ErrNoEvidence
	}
	if echoedNonce != r.nonce {
		return false, ErrNonceMismatch
	}
	return approve, nil
}
