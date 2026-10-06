package netpolicy_test

import (
	"errors"
	"net"
	"testing"

	"github.com/t0ul/ai-security-engineering/netpolicy"
)

// fakeDNS maps hostnames to IPs so Check is deterministic without real DNS.
func fakeDNS(m map[string]string) func(string) ([]net.IP, error) {
	return func(host string) ([]net.IP, error) {
		if s, ok := m[host]; ok {
			return []net.IP{net.ParseIP(s)}, nil
		}
		return nil, &net.DNSError{Err: "no such host", Name: host}
	}
}

func TestAllowsAllowlistedPublicHost(t *testing.T) {
	p := netpolicy.Policy{Allow: []string{"api.example.com"}, Resolve: fakeDNS(map[string]string{"api.example.com": "93.184.216.34"})}
	if err := p.Check("https://api.example.com/v1/search?q=x"); err != nil {
		t.Fatalf("allowlisted public host should pass, got %v", err)
	}
}

func TestBlocksIMDS(t *testing.T) {
	p := netpolicy.Policy{Allow: []string{"metadata"}, Resolve: fakeDNS(map[string]string{"metadata": "169.254.169.254"})}
	if err := p.Check("http://metadata/latest/meta-data/iam/security-credentials/"); !errors.Is(err, netpolicy.ErrBlockedIP) {
		t.Fatalf("IMDS link-local must be blocked, got %v", err)
	}
}

func TestBlocksPrivateAndLoopback(t *testing.T) {
	p := netpolicy.Policy{Allow: []string{"h"}}
	p.Resolve = fakeDNS(map[string]string{"h": "10.0.0.5"})
	if err := p.Check("http://h/"); !errors.Is(err, netpolicy.ErrBlockedIP) {
		t.Fatalf("private IP must be blocked, got %v", err)
	}
	p.Resolve = fakeDNS(map[string]string{"h": "127.0.0.1"})
	if err := p.Check("http://h/"); !errors.Is(err, netpolicy.ErrBlockedIP) {
		t.Fatalf("loopback must be blocked by default, got %v", err)
	}
	p.AllowLoopback = true
	if err := p.Check("http://h/"); err != nil {
		t.Fatalf("loopback should pass when AllowLoopback set, got %v", err)
	}
}

func TestDefaultDenyNotAllowlisted(t *testing.T) {
	p := netpolicy.Policy{Allow: []string{"good.com"}, Resolve: fakeDNS(map[string]string{"evil.com": "93.184.216.34"})}
	if err := p.Check("https://evil.com/"); !errors.Is(err, netpolicy.ErrNotAllowlisted) {
		t.Fatalf("non-allowlisted host must be denied, got %v", err)
	}
}

func TestRejectsNonHTTPScheme(t *testing.T) {
	p := netpolicy.Policy{Allow: []string{"x"}}
	if err := p.Check("file:///etc/passwd"); !errors.Is(err, netpolicy.ErrScheme) {
		t.Fatalf("non-http scheme must be rejected, got %v", err)
	}
}

func TestBlocksDNSRebinding(t *testing.T) {
	// Host is allowlisted but resolves to an internal IP — the rebinding attack.
	p := netpolicy.Policy{Allow: []string{"good.test"}, Resolve: fakeDNS(map[string]string{"good.test": "10.1.2.3"})}
	if err := p.Check("https://good.test/"); !errors.Is(err, netpolicy.ErrBlockedIP) {
		t.Fatalf("allowlisted host resolving internal must be blocked (rebinding), got %v", err)
	}
}
