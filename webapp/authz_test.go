package webapp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/t0ul/ai-security-engineering/controlplane"
	"github.com/t0ul/ai-security-engineering/pkg/provenance"
)

// authzFixture returns a Server with authZ enabled plus a freshly minted
// operator token (base64url Grant) scoped broadly.
func authzFixture(t *testing.T) (*Server, string, *controlplane.Authority) {
	t.Helper()
	signer, pub, err := provenance.NewSigner("agent")
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	authority := controlplane.NewAuthority(signer, provenance.NewVerifier().Trust("agent", pub))
	g, err := authority.Issue(controlplane.Capability{Subject: "operator", Action: controlplane.Scope, Resource: controlplane.Scope, Tenant: controlplane.Scope}, time.Hour)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	return &Server{Authz: authority}, EncodeToken(g), authority
}

// gate wraps a trivial 200 handler so we test the capability check in isolation,
// independent of any real endpoint's downstream behavior.
func gate(s *Server, action, resource string) http.HandlerFunc {
	return s.authz(action, resource, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func call(h http.HandlerFunc, token string) int {
	r := httptest.NewRequest(http.MethodPost, "/api/accept", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h(w, r)
	return w.Code
}

func TestAuthzNilIsNoOp(t *testing.T) {
	s := &Server{} // Authz unset = household mode
	if code := call(gate(s, controlplane.ActionWrite, "calendar"), ""); code != http.StatusOK {
		t.Fatalf("no-op gate should pass without a token, got %d", code)
	}
}

func TestAuthzMissingToken(t *testing.T) {
	s, _, _ := authzFixture(t)
	if code := call(gate(s, controlplane.ActionWrite, "calendar"), ""); code != http.StatusUnauthorized {
		t.Fatalf("missing token should be 401, got %d", code)
	}
}

func TestAuthzValidOperatorToken(t *testing.T) {
	s, tok, _ := authzFixture(t)
	// The broad operator grant covers write/calendar, export/link, and list/corpus.
	for _, sc := range []struct{ action, resource string }{
		{controlplane.ActionWrite, "calendar"},
		{controlplane.ActionExport, "link"},
		{controlplane.ActionList, "corpus"},
	} {
		if code := call(gate(s, sc.action, sc.resource), tok); code != http.StatusOK {
			t.Fatalf("operator token should pass %s/%s, got %d", sc.action, sc.resource, code)
		}
	}
}

func TestAuthzOverScopedTokenRefused(t *testing.T) {
	s, _, authority := authzFixture(t)
	// A grant scoped only to read the calendar must not drive a write or an export.
	g, _ := authority.Issue(controlplane.Capability{Subject: "viewer", Action: controlplane.ActionRead, Resource: "calendar", Tenant: "public"}, time.Hour)
	tok := EncodeToken(g)
	if code := call(gate(s, controlplane.ActionWrite, "calendar"), tok); code != http.StatusForbidden {
		t.Fatalf("read grant must not authorize a write, got %d", code)
	}
	if code := call(gate(s, controlplane.ActionExport, "link"), tok); code != http.StatusForbidden {
		t.Fatalf("read-calendar grant must not authorize link export, got %d", code)
	}
}

// TestAuthzHaltRevokesInFlight is the kill-switch-as-revocation invariant at the
// HTTP layer: a valid operator token stops authorizing the instant Halt engages.
func TestAuthzHaltRevokesInFlight(t *testing.T) {
	s, tok, authority := authzFixture(t)
	halted := false
	authority.Halted = func() bool { return halted }
	h := gate(s, controlplane.ActionWrite, "calendar")
	if code := call(h, tok); code != http.StatusOK {
		t.Fatalf("token should pass before halt, got %d", code)
	}
	halted = true
	if code := call(h, tok); code != http.StatusForbidden {
		t.Fatalf("halt must revoke the in-flight token, got %d", code)
	}
}
