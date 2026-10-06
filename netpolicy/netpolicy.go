// Package netpolicy is the M16 egress guard: the control that neutralizes
// post-injection exfiltration and SSRF even when the model is compromised. A
// tool that fetches a URL (web_fetch/browse) runs every target through Check,
// which enforces a default-deny host allowlist and refuses any host that
// resolves to a loopback, link-local (incl. the cloud-metadata IP
// 169.254.169.254), private, or unspecified address. Resolution happens here, so
// a name that passes the allowlist but resolves to an internal IP — DNS
// rebinding — is still blocked.
package netpolicy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrScheme        = errors.New("netpolicy: only http and https are allowed")
	ErrNoHost        = errors.New("netpolicy: URL has no host")
	ErrNotAllowlisted = errors.New("netpolicy: host is not on the egress allowlist")
	ErrBlockedIP     = errors.New("netpolicy: host resolves to a blocked (internal) address")
)

// Policy is an egress allowlist. The zero value denies everything.
type Policy struct {
	// Allow is the set of permitted hostnames (exact, case-insensitive).
	// Default-deny: a host not listed is refused.
	Allow []string
	// AllowLoopback permits 127.0.0.1/::1 — dev-only, e.g. to reach a local test
	// target. Never set it in production; loopback is an SSRF surface.
	AllowLoopback bool
	// Resolve overrides DNS resolution (tests inject a fake). Defaults to
	// net.LookupIP.
	Resolve func(host string) ([]net.IP, error)
}

func (p Policy) resolve() func(string) ([]net.IP, error) {
	if p.Resolve != nil {
		return p.Resolve
	}
	return net.LookupIP
}

// Check reports whether rawURL may be fetched. It validates the scheme, enforces
// the host allowlist, then resolves the host and rejects any internal address.
func (p Policy) Check(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("netpolicy: parse %q: %w", rawURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w (got %q)", ErrScheme, u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return ErrNoHost
	}
	if !p.hostAllowed(host) {
		return fmt.Errorf("%w: %q", ErrNotAllowlisted, host)
	}
	ips, err := p.resolve()(host)
	if err != nil {
		return fmt.Errorf("netpolicy: resolve %q: %w", host, err)
	}
	for _, ip := range ips {
		if blocked(ip, p.AllowLoopback) {
			return fmt.Errorf("%w: %s -> %s", ErrBlockedIP, host, ip)
		}
	}
	return nil
}

// DialContext resolves the host once, validates every candidate IP, and connects
// only to a vetted one. Because the same resolution both validates and dials,
// there is no window for DNS rebinding between a check and the connection — this
// is the authoritative egress enforcement (Check is a fast pre-filter).
func (p Policy) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if !p.hostAllowed(host) {
		return nil, fmt.Errorf("%w: %q", ErrNotAllowlisted, host)
	}
	ips, err := p.resolve()(host)
	if err != nil {
		return nil, fmt.Errorf("netpolicy: resolve %q: %w", host, err)
	}
	var d net.Dialer
	for _, ip := range ips {
		if blocked(ip, p.AllowLoopback) {
			continue
		}
		if conn, derr := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port)); derr == nil {
			return conn, nil
		}
	}
	return nil, fmt.Errorf("%w: %q", ErrBlockedIP, host)
}

// HTTPClient returns an http.Client that can only connect to allow-listed hosts
// resolving to non-internal IPs — safe against SSRF and DNS rebinding.
func (p Policy) HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: p.DialContext},
	}
}

func (p Policy) hostAllowed(host string) bool {
	for _, a := range p.Allow {
		if strings.EqualFold(a, host) {
			return true
		}
	}
	return false
}

// blocked reports whether an IP is in an internal range an egress tool must
// never reach: loopback (unless explicitly permitted), link-local (covers the
// IMDS 169.254.169.254 and IPv6 fe80::/10), private (RFC1918 / ULA), and the
// unspecified address.
func blocked(ip net.IP, allowLoopback bool) bool {
	if ip.IsLoopback() {
		return !allowLoopback
	}
	return ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsPrivate() ||
		ip.IsUnspecified()
}
