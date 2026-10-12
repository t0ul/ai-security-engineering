// Command main is the runnable demo for lesson 03.2, a gateway you cannot route
// around.
//
// Run it with:
//
//	go run .
//
// Every model call should pass through one choke point. A naive proxy that lets
// the caller choose the upstream is a server-side request forgery waiting to
// happen, and with no caps it is a flood waiting to happen. This demo fires both
// attacks at a naive proxy and at the real gouncer gateway. Everything runs on
// throwaway local servers; the flood is a bounded burst of ten requests, never a
// real service.
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/t0ul/gouncer"
)

func main() {
	// A legitimate model backend (the only place a call should go) and an internal
	// service the agent must never reach through a model call.
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"[]"}}]}`)
	}))
	defer backend.Close()

	internalHits := 0
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHits++
		io.WriteString(w, "INTERNAL-SECRET: db_password=hunter2")
	}))
	defer internal.Close()

	ssrf := ssrfBlocked(backend.URL, internal.URL, &internalHits)
	flood := floodBounded(backend.URL)
	body := bodyCapped(backend.URL)

	fmt.Println()
	if ssrf && flood && body {
		fmt.Println("attack blocked (the receipt)")
	} else {
		fmt.Println("a control did not hold")
		os.Exit(1)
	}
}

// ---- the two proxies ----

// naiveProxy is the gateway I would write first: it reads the upstream to call
// from the request and forwards there, with no allowlist and no caps. Letting the
// caller pick the host is the SSRF; having no caps is the flood.
func naiveProxy() http.HandlerFunc {
	client := &http.Client{}
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstream := extractUpstream(body) // the caller's chosen host, trusted
		resp, err := client.Post(upstream+"/v1/chat/completions", "application/json", bytes.NewReader(body))
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

// gouncerGateway is the real control: an allowlist of logical model names, a body
// cap, and a per-client rate cap. The caller can never name a host, only a model,
// and an unknown model is refused.
func gouncerGateway(backendURL string) http.Handler {
	gw, err := gouncer.New(gouncer.Config{
		Routes:        map[string]gouncer.Route{"planner": {Upstream: backendURL}},
		MaxBodyBytes:  512,
		RatePerMinute: 60,
		RateBurst:     3,
	})
	if err != nil {
		panic(err)
	}
	return gw
}

// ---- attack 1: SSRF ----

func ssrfBlocked(backendURL, internalURL string, hits *int) bool {
	fmt.Println("== SSRF: a model call aimed at an internal service ==")

	// Undefended: the attacker names the internal service as the upstream.
	np := httptest.NewServer(naiveProxy())
	defer np.Close()
	*hits = 0
	req := fmt.Sprintf(`{"model":"planner","upstream":%q,"messages":[]}`, internalURL)
	code, _ := post(np.URL, "attacker", req)
	leaked := *hits > 0
	fmt.Printf("  naive proxy:  HTTP %d, internal service hit %d time(s)%s\n", code, *hits,
		tag(leaked, "  <- SSRF: reached the internal host and leaked its secret"))

	// Defended: gouncer takes no upstream from the caller; the attacker can only
	// name a model, and the internal service is not one.
	gw := httptest.NewServer(gouncerGateway(backendURL))
	defer gw.Close()
	*hits = 0
	code2, body2 := post(gw.URL, "attacker-ssrf", `{"model":"internal","upstream":"`+internalURL+`","messages":[]}`)
	fmt.Printf("  gouncer:      HTTP %d %s, internal service hit %d time(s)\n", code2, errMsg(body2), *hits)
	return leaked && code2 == http.StatusBadRequest && *hits == 0
}

// ---- attack 2: flood (bounded) ----

func floodBounded(backendURL string) bool {
	fmt.Println("\n== flood: a bounded burst of 10 requests ==")
	const burst = 10

	np := httptest.NewServer(naiveProxy())
	defer np.Close()
	served := 0
	for i := 0; i < burst; i++ {
		if code, _ := post(np.URL, "flood", fmt.Sprintf(`{"model":"planner","upstream":%q,"messages":[]}`, backendURL)); code == 200 {
			served++
		}
	}
	fmt.Printf("  naive proxy:  %d/%d served, 0 refused (nothing says stop)\n", served, burst)

	gw := httptest.NewServer(gouncerGateway(backendURL))
	defer gw.Close()
	ok, limited := 0, 0
	for i := 0; i < burst; i++ {
		switch code, _ := post(gw.URL, "flood", `{"model":"planner","messages":[]}`); code {
		case 200:
			ok++
		case http.StatusTooManyRequests:
			limited++
		}
	}
	fmt.Printf("  gouncer:      %d served, %d refused with 429 once past the rate cap\n", ok, limited)
	return served == burst && limited > 0
}

// ---- attack 3: oversized body ----

func bodyCapped(backendURL string) bool {
	fmt.Println("\n== oversized body ==")
	big := `{"model":"planner","messages":[{"role":"user","content":"` + repeat("A", 4096) + `"}]}`

	np := httptest.NewServer(naiveProxy())
	defer np.Close()
	code, _ := post2(np.URL, "body", `{"model":"planner","upstream":"`+backendURL+`","messages":[{"role":"user","content":"`+repeat("A", 4096)+`"}]}`)
	fmt.Printf("  naive proxy:  HTTP %d (a 4 KB body sailed through)\n", code)

	gw := httptest.NewServer(gouncerGateway(backendURL))
	defer gw.Close()
	code2, body2 := post2(gw.URL, "body", big)
	fmt.Printf("  gouncer:      HTTP %d %s (cap is 512 bytes)\n", code2, errMsg(body2))
	return code == 200 && code2 == http.StatusRequestEntityTooLarge
}

// ---- helpers ----

func post(base, key, body string) (int, string) { return post2(base, key, body) }

func post2(base, key, body string) (int, string) {
	req, _ := http.NewRequest("POST", base+"/v1/chat/completions", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", key) // give each scenario its own rate bucket
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func extractUpstream(body []byte) string {
	s := string(body)
	i := bytes_Index(s, `"upstream":"`)
	if i < 0 {
		return ""
	}
	s = s[i+len(`"upstream":"`):]
	j := bytes_Index(s, `"`)
	if j < 0 {
		return ""
	}
	return s[:j]
}

func bytes_Index(s, sub string) int { return bytes.Index([]byte(s), []byte(sub)) }

func errMsg(body string) string {
	if i := bytes_Index(body, `"message":"`); i >= 0 {
		s := body[i+len(`"message":"`):]
		if j := bytes_Index(s, `"`); j >= 0 {
			return "(" + s[:j] + ")"
		}
	}
	return ""
}

func tag(cond bool, s string) string {
	if cond {
		return s
	}
	return ""
}

func repeat(s string, n int) string {
	out := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, s[0])
	}
	return string(out)
}
