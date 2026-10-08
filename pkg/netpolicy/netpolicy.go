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
	ErrScheme         = errors.New("netpolicy: only http and https are allowed")
	ErrNoHost         = errors.New("netpolicy: URL has no host")
	ErrNotAllowlisted = errors.New("netpolicy: host is not on the egress allowlist")
	ErrBlockedIP      = errors.New("netpolicy: host resolves to a blocked (internal) address")
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
// resolving to non-internal IPs — safe against SSRF and DNS rebinding. Redirects
// are re-validated per hop (a 302 to an internal host is refused), and the
// transport dials only vetted IPs, so neither DNS rebinding nor a redirect can
// steer the request to an internal address.
func (p Policy) HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &http.Transport{DialContext: p.DialContext},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("netpolicy: too many redirects")
			}
			return p.Check(req.URL.String())
		},
	}
}

// hostAllowed matches an allowlist entry exactly, or as a parent domain: an
// entry "schools.nyc.gov" also permits its subdomains ("www.schools.nyc.gov").
// Internal-IP resolution is still rejected afterward, so a permissive domain
// cannot be abused to reach an internal address via a subdomain.
func (p Policy) hostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, a := range p.Allow {
		a = strings.ToLower(strings.TrimPrefix(a, "."))
		if a == "" {
			continue
		}
		if host == a || strings.HasSuffix(host, "."+a) {
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
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() ||
		ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}
	// Ranges Go's helpers don't flag but which must not be egress targets:
	// CGNAT 100.64.0.0/10, IETF benchmarking 198.18.0.0/15, limited broadcast.
	if v4 := ip.To4(); v4 != nil {
		switch {
		case v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127:
			return true
		case v4[0] == 198 && (v4[1] == 18 || v4[1] == 19):
			return true
		case v4[0] == 255 && v4[1] == 255 && v4[2] == 255 && v4[3] == 255:
			return true
		}
	}
	return false
}
