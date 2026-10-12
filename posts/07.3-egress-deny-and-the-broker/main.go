// Command main is the runnable demo for lesson 07.3, egress-deny and the one way
// out.
//
// Run it with:
//
//	go run .
//
// The detonation chamber has no network card, so code inside it cannot reach the
// network on its own. When a tool legitimately needs to, it asks a broker on the
// host to open the connection. This demo asks a naive broker that dials whatever it
// is told, and reaches an internal service it should not, then the agent's real
// broker, which gates every target through the egress policy and refuses the
// internal one while still allowing a legitimate host. Deterministic, no model.
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/t0ul/emaildrop/pkg/broker"
	"github.com/t0ul/emaildrop/pkg/netpolicy"
)

var internalHits int

func main() {
	// A legitimate host the tool is allowed to use, and an internal service it must
	// never reach through the broker.
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "ALLOWED: this week's weather")
	}))
	defer ok.Close()
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHits++
		io.WriteString(w, "INTERNAL-SECRET: db credentials")
	}))
	defer internal.Close()

	okTarget := "weather.example:" + port(ok.URL)
	internalTarget := "internal.example:" + port(internal.URL)

	landed := naiveReachesInternal(internalTarget)
	blocked := brokerGates(okTarget, internalTarget)

	fmt.Println()
	if landed && blocked {
		fmt.Println("attack blocked (the receipt): the naive broker dials anything; the real one dials only the allowlist")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// resolve is a fixed resolver: the demo's named hosts map to the loopback test
// servers; everything else resolves normally.
func resolve(host string) ([]net.IP, error) {
	switch host {
	case "weather.example", "internal.example":
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	return net.LookupIP(host)
}

// ---- before: a broker that dials whatever it is asked ----

// naiveBroker is the egress helper written without a policy: read the target, dial
// it, tunnel. It is the chamber's one door, propped wide open.
func naiveBroker(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			line := make([]byte, 256)
			n, _ := conn.Read(line)
			target := strings.TrimSpace(string(line[:n]))
			// No policy, but it still has to resolve the name to dial it, the same
			// way the real broker does; the difference is it asks no questions.
			host, prt, _ := net.SplitHostPort(target)
			ips, _ := resolve(host)
			up, err := net.Dial("tcp", net.JoinHostPort(ips[0].String(), prt)) // no check at all
			if err != nil {
				fmt.Fprintf(conn, "ERR %v\n", err)
				return
			}
			defer up.Close()
			io.WriteString(conn, "OK\n")
			go io.Copy(up, conn)
			io.Copy(conn, up)
		}()
	}
}

func naiveReachesInternal(internalTarget string) bool {
	fmt.Println("== before: a broker with no policy ==")
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go naiveBroker(ln)

	internalHits = 0
	body, err := guestFetch(ln.Addr().String(), "http://"+internalTarget+"/")
	fmt.Printf("  the guest asked the broker for %s\n  it got back: %q (internal service hit %d time(s))\n",
		internalTarget, body, internalHits)
	_ = err
	return internalHits > 0
}

// ---- after: the real broker gates every target ----

func brokerGates(okTarget, internalTarget string) bool {
	fmt.Println("\n== after: the agent's real broker ==")
	b := &broker.Broker{
		Policy:  netpolicy.Policy{Allow: []string{"weather.example"}, AllowLoopback: true, Resolve: resolve},
		Timeout: 5 * time.Second,
	}
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln.Close()
	go b.Serve(ln)

	// The forbidden internal host: refused before any connection is made.
	internalHits = 0
	_, errInt := guestFetch(ln.Addr().String(), "http://"+internalTarget+"/")
	fmt.Printf("  internal.example: %s (internal service hit %d time(s))\n", refusal(errInt), internalHits)

	// The allowlisted host: tunneled through, and the tool gets its data.
	okBody, errOk := guestFetch(ln.Addr().String(), "http://"+okTarget+"/")
	fmt.Printf("  weather.example:  %s\n", result(okBody, errOk))

	return internalHits == 0 && errInt != nil && errOk == nil && strings.Contains(okBody, "ALLOWED")
}

// guestFetch is the in-chamber tool: it reaches the network ONLY by asking the
// broker, using the broker's own HTTP client.
func guestFetch(brokerAddr, url string) (string, error) {
	dial := func(ctx context.Context) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", brokerAddr)
	}
	client := broker.HTTPClient(dial, 5*time.Second)
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func port(u string) string { return u[strings.LastIndexByte(u, ':')+1:] }

func refusal(err error) string {
	if err == nil {
		return "REACHED (should not happen)"
	}
	return "refused"
}

func result(body string, err error) string {
	if err != nil {
		return "error: " + err.Error()
	}
	return "tunneled, got: " + body
}
