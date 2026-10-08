package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

// enrich fetches a handbook/reference URL into the knowledge base (R6). Security
// by construction, same spine as an action link: the URL is cleared by the egress
// allowlist (netpolicy deny-by-default, registrable-domain, no loopback/private/
// IMDS), gated by an evidence-first single-use HITL confirm, fetched INSIDE the
// MicroVM (never on the host), and indexed as UNTRUSTED provenance (+ PII-scrubbed
// on ingest) so Ask-School can answer from it without ever trusting it as
// instructions. Never auto-fetched.
func (s *Server) enrich(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		URL     string `json:"url"`
		Nonce   string `json:"nonce"`
		Confirm bool   `json:"confirm"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || !isHTTPURL(req.URL) {
		http.Error(w, "an http(s) url is required", http.StatusBadRequest)
		return
	}
	if s.toolsBlocked() {
		http.Error(w, "blocked by the kill switch", http.StatusServiceUnavailable)
		return
	}
	// HITL (ASI09): a handbook URL is untrusted — fetch only on an evidence-first,
	// single-use confirm. The nonce is bound to this exact URL.
	if !s.hitlGate(w, "enrich", req.URL, 0, req.Nonce, req.Confirm,
		"Fetch a link into the knowledge base", "url: "+req.URL) {
		return
	}
	if err := s.egressPolicy().Check(req.URL); err != nil {
		writeJSON(w, map[string]any{"ok": false, "url": req.URL, "refused": err.Error()})
		return
	}
	if s.Fetch == nil || s.Index == nil {
		writeJSON(w, map[string]any{"ok": false, "url": req.URL, "refused": "enrichment not configured"})
		return
	}
	text, _ := s.Fetch(r.Context(), req.URL)
	if err := s.Index("web:"+req.URL, text); err != nil {
		writeJSON(w, map[string]any{"ok": false, "url": req.URL, "refused": "index failed: " + err.Error()})
		return
	}
	preview := text
	if len(preview) > 600 {
		preview = preview[:600] + "\n…(truncated)"
	}
	writeJSON(w, map[string]any{"ok": true, "url": req.URL, "bytes": len(text), "indexed": true, "preview": preview})
}

func isHTTPURL(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://")
}
