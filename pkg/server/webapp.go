// Package webapp is the local operator web app for managing the agent: run the
// security scorecard from the UI, and browse + replay incidents from the
// chain-verified audit log. It reuses the same Go components the agent runs on
// (redteam, ir), so the console and the system under test are one binary.
package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/t0ul/ai-security-engineering/pkg/controlplane"
	"github.com/t0ul/ai-security-engineering/pkg/ir"
	"github.com/t0ul/ai-security-engineering/pkg/netpolicy"
	"github.com/t0ul/ai-security-engineering/pkg/provenance"
	"github.com/t0ul/ai-security-engineering/pkg/redteam"
	"github.com/t0ul/ai-security-engineering/ui"
	"github.com/t0ul/gledger"
	"github.com/t0ul/gorauder"
)

// Server serves the dashboard and its API.
type Server struct {
	AuditPath string // gledger log to read incidents from
	OutboxDir string // accepted .ics artifacts to render + offer for download
	InboxPath string // where the user drops .txt emails (shown in the UI)

	// Egress is the action-link allowlist (deny-by-default). Fetch performs the
	// vetted fetch — in production controlplane.SandboxFetchTool (in the MicroVM).
	// Both unset = action links are refused.
	Egress netpolicy.Policy
	Fetch  Fetcher
	// Verifier, when set, checks each .ics against its .sig content credential so
	// the UI can show whether the agent provably produced it unaltered (M20).
	Verifier *provenance.Verifier
	// Safety, when set, is the layered kill switch (M18): Pause refuses new
	// processing/drops; BlockTools refuses accept/action side effects.
	Safety *controlplane.Safety
	// Search, when set, answers Ask-School queries over the (PII-scrubbed)
	// retrieval corpus. Nil = the Ask tab returns nothing.
	Search func(query string, k int) ([]SearchHit, error)
	// Index, when set with Fetch, powers safe handbook link enrichment (R6): it
	// adds fetched (untrusted, PII-scrubbed) text to the retrieval corpus.
	Index func(source, text string) error
	// Feedback / FlywheelStats, when set, capture operator accept/reject decisions
	// as durable ground-truth (the data-flywheel) and report the running tallies.
	Feedback      func(decision, source, title string)
	FlywheelStats func() (accepts, rejects int)
	// Chat, when set, answers a question grounded in the corpus. unsafe=true runs
	// the UNDEFENDED path (raw-concat retrieval, no encapsulation/scrub) — the live
	// attack demo showing a poisoned doc's injection land; default is defended.
	Chat func(question string, unsafe bool) (answer string, sources []string, err error)

	// Authz, when set, turns the console into an authZ'd API: every mutating or
	// data-listing endpoint requires a capability Grant (C4b). The operator
	// holds one broad grant (OperatorToken); the agent and tools get narrower
	// ones. Nil = the gate is a no-op, so the loopback household app runs
	// unauthenticated. Verify is fail-closed and the kill switch (Authority.
	// Halted) revokes every grant, so Halt kills in-flight side effects too.
	Authz *controlplane.Authority
	// OperatorToken is the base64url(JSON) operator Grant injected into the page
	// so the browser presents it on every /api and /ics request. Empty = none.
	OperatorToken string
	// Audit, when set, records every refused capability for the admin trail.
	Audit *gledger.AuditLog

	// MCP, when set, lists the MCP servers the agent reaches through the gustoms
	// gateway with their pin status (C6). MCPApprove re-pins a server to its
	// current manifest (rug-pull recovery, dual-control). Nil = no MCP tab.
	MCP        func() []MCPServer
	MCPApprove func(server string) error

	// Prompts, when set, is the governed prompt resolver (C2): the console lists
	// each model's active/default system prompt, activates a new versioned+hashed
	// version, and rolls back to the shipped default. Nil = no Prompts tab.
	Prompts *controlplane.Prompts

	// Budgets, when set, is the governed budgets/limits resolver (C9): rate/token/
	// concurrency/spend ceilings + key POINTERS (env-var names, never values). The
	// "api" budget's RatePerMin is enforced on the console API here; the rest are
	// gateway-side. Nil = no Budgets tab, no rate limit.
	Budgets  *controlplane.Budgets
	rlMu     sync.Mutex
	rlCount  int
	rlWindow time.Time

	// Sampling, when set, is the governed decoding-params resolver (C5):
	// temperature/max_tokens/seed per model as versioned, hashed, rollback-able
	// artifacts. Nil = no Sampling tab.
	Sampling *controlplane.Sampling

	// Policies, when set, is the governed policy-allowlist resolver (C8): egress /
	// exec / guardrail allowlists as versioned, hashed, rollback-able artifacts.
	// The action egress check resolves its allowlist from here at request time, so
	// a governed change takes effect with no redeploy. Nil = no Policies tab and
	// the static Egress is used.
	Policies *controlplane.Policies

	// ProfileLoad/ProfileSave, when set, read/write the household child profile
	// (R1) in the governed DB — config lives in the store, not a file or a const.
	// Operator-set, host-only, never from an email. Enables the My Week tab.
	ProfileLoad func() Profile
	ProfileSave func(Profile) error

	// Bundle surfaces (C10): BundleList shows saved known-good snapshots; BundleSave
	// snapshots the whole governed plane; BundleApply rolls it all back. Nil = no
	// snapshot card.
	BundleList  func() []BundleInfo
	BundleSave  func(label string) error
	BundleApply func(label string) error

	// Eval surfaces (C7): EvalHistory lists persisted F1 scores (the trend card);
	// EvalRun runs the eval live and persists it; PromptTest shadow-evals a
	// candidate extractor prompt (F1 + ADD-ASR + promotion-gate verdict) WITHOUT
	// activating it. Nil = no Eval tab / Test button.
	EvalHistory func() []EvalResult
	EvalRun     func() ([]EvalResult, string)
	PromptTest  func(name, candidate string) PromptTestResult

	// pending holds issued-but-unconfirmed HITL approvals, keyed by nonce (ASI09:
	// evidence-first, single-use, clickjack/forgery-resistant confirm).
	mu   sync.Mutex
	pend map[string]pending
}

// verifySig reports whether <name>.sig is a valid content credential over the
// given .ics bytes from a trusted agent key.
func (s *Server) verifySig(name string, content []byte) bool {
	if s.Verifier == nil {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(s.OutboxDir, name+".sig"))
	if err != nil {
		return false
	}
	var m provenance.Mark
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	return s.Verifier.Verify(content, m) == nil
}

// Handler returns the app routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/scorecard", s.scorecard)
	mux.HandleFunc("/api/incidents", s.incidents)
	mux.HandleFunc("/api/incident", s.incident)
	mux.HandleFunc("/api/events", s.events)
	mux.HandleFunc("/api/items", s.items)
	mux.HandleFunc("/api/summary", s.summary)
	mux.HandleFunc("/api/review", s.review)
	mux.HandleFunc("/api/activity", s.activity)
	mux.HandleFunc("/api/safety", s.safetyState)
	// The kill switch is deliberately OUTSIDE the capability gate: engaging Halt
	// revokes every grant, so a gated killswitch could never be disengaged.
	mux.HandleFunc("/api/killswitch", csrf(s.killswitch))
	// Ask lists the retrieval corpus — ActionList, the data-residency-sensitive
	// endpoint (a frontier reader must not harvest it).
	mux.HandleFunc("/api/ask", s.authz(controlplane.ActionList, "corpus", s.ask))
	if s.Chat != nil {
		mux.HandleFunc("/api/chat", csrf(s.authz(controlplane.ActionList, "corpus", s.chat)))
	}
	if s.MCP != nil {
		mux.HandleFunc("/api/mcp", s.mcpList)
		mux.HandleFunc("/api/mcp/approve", csrf(s.authz(controlplane.ActionWrite, "mcp", s.mcpApprove)))
	}
	if s.Prompts != nil {
		mux.HandleFunc("/api/prompts", s.promptsList)
		mux.HandleFunc("/api/prompts/activate", csrf(s.authz(controlplane.ActionWrite, "prompts", s.promptsActivate)))
		mux.HandleFunc("/api/prompts/reset", csrf(s.authz(controlplane.ActionWrite, "prompts", s.promptsReset)))
	}
	if s.Budgets != nil {
		mux.HandleFunc("/api/budgets", s.budgetsList)
		mux.HandleFunc("/api/budgets/activate", csrf(s.authz(controlplane.ActionWrite, "budget", s.budgetsActivate)))
		mux.HandleFunc("/api/budgets/reset", csrf(s.authz(controlplane.ActionWrite, "budget", s.budgetsReset)))
	}
	if s.Sampling != nil {
		mux.HandleFunc("/api/sampling", s.samplingList)
		mux.HandleFunc("/api/sampling/activate", csrf(s.authz(controlplane.ActionWrite, "sampling", s.samplingActivate)))
		mux.HandleFunc("/api/sampling/reset", csrf(s.authz(controlplane.ActionWrite, "sampling", s.samplingReset)))
	}
	if s.Policies != nil {
		mux.HandleFunc("/api/policies", s.policiesList)
		mux.HandleFunc("/api/policies/activate", csrf(s.authz(controlplane.ActionWrite, "policy", s.policiesActivate)))
		mux.HandleFunc("/api/policies/reset", csrf(s.authz(controlplane.ActionWrite, "policy", s.policiesReset)))
	}
	if s.FlywheelStats != nil {
		mux.HandleFunc("/api/flywheel", s.flywheel)
	}
	if s.EvalHistory != nil {
		mux.HandleFunc("/api/eval", s.evalList)
		mux.HandleFunc("/api/eval/run", csrf(s.authz(controlplane.ActionWrite, "eval", s.evalRunHandler)))
	}
	if s.PromptTest != nil {
		mux.HandleFunc("/api/prompts/test", csrf(s.authz(controlplane.ActionWrite, "prompts", s.promptsTest)))
	}
	if s.BundleList != nil {
		mux.HandleFunc("/api/bundles", s.bundlesList)
		mux.HandleFunc("/api/bundles/save", csrf(s.authz(controlplane.ActionWrite, "bundle", s.bundlesSave)))
		mux.HandleFunc("/api/bundles/apply", csrf(s.authz(controlplane.ActionWrite, "bundle", s.bundlesApply)))
	}
	if s.ProfileLoad != nil {
		mux.HandleFunc("/api/timeline", s.timeline)
		mux.HandleFunc("/api/profile", s.profileGet)
		mux.HandleFunc("/api/profile/save", csrf(s.authz(controlplane.ActionWrite, "profile", s.profileSave)))
	}
	if s.InboxPath != "" {
		mux.HandleFunc("/api/drop", csrf(s.authz(controlplane.ActionWrite, "inbox", s.drop)))
	}
	if s.OutboxDir != "" {
		mux.HandleFunc("/api/accept", csrf(s.authz(controlplane.ActionWrite, "calendar", s.accept)))
		mux.HandleFunc("/api/reject", csrf(s.authz(controlplane.ActionWrite, "calendar", s.reject)))
		mux.HandleFunc("/api/action", csrf(s.authz(controlplane.ActionExport, "link", s.action))) // egress
	}
	if s.Fetch != nil && s.Index != nil {
		mux.HandleFunc("/api/enrich", csrf(s.authz(controlplane.ActionExport, "link", s.enrich))) // egress → corpus
	}
	if s.OutboxDir != "" {
		mux.HandleFunc("/ics/", s.serveICS)           // .ics only — NOT the whole outbox (sidecars hold PII)
		mux.HandleFunc("/api/directory", s.directory) // consolidated contacts (R5)
	}
	mux.HandleFunc("/", s.index)
	return mux
}

// serveICS serves ONLY .ics artifacts from the outbox by bare basename. The
// outbox also holds <stem>.summary.json sidecars (contacts/digest/PII), so a
// blanket file server would leak them.
func (s *Server) serveICS(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/ics/")
	if name == "" || filepath.Base(name) != name || !strings.HasSuffix(strings.ToLower(name), ".ics") {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, filepath.Join(s.OutboxDir, name))
}

// csrf rejects cross-site POSTs. The app binds loopback, but that does NOT stop
// CSRF — any site the browser visits can POST to 127.0.0.1. A foreign Origin is
// refused; same-origin fetches (loopback Origin, or no Origin) pass.
func csrf(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" {
			u, err := url.Parse(o)
			if err != nil || !isLoopbackHost(u.Hostname()) {
				http.Error(w, "cross-origin request refused", http.StatusForbidden)
				return
			}
		}
		h(w, r)
	}
}

func isLoopbackHost(h string) bool {
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

// egressPolicy resolves the action-link allowlist live from the governed Policies
// (C8) when set, keeping every other netpolicy field (DNS pinning, the
// private/IMDS deny-by-default) intact — a governed widening still cannot reach
// link-local/RFC1918. Falls back to the static Egress.
func (s *Server) egressPolicy() netpolicy.Policy {
	p := s.Egress
	if s.Policies != nil {
		p.Allow = s.Policies.Items("egress")
	}
	return p
}

// authz gates a handler on a capability Grant scoped to {action, resource,
// tenant "public"}. When Authz is unset the gate is a no-op (the household
// loopback app runs unauthenticated). The caller presents the Grant as
// "Authorization: Bearer <base64url(JSON)>". Fail-closed: a missing, malformed,
// unsigned, expired, out-of-scope, or revoked Grant is refused, and Authority.
// Halted (the kill switch) revokes all grants so Halt stops in-flight work.
func (s *Server) authz(action, resource string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Authz == nil {
			h(w, r)
			return
		}
		g, ok := bearerGrant(r)
		if !ok {
			s.denyAudit(action, resource, "missing capability")
			http.Error(w, "missing capability", http.StatusUnauthorized)
			return
		}
		if _, err := s.Authz.Verify(g, controlplane.Capability{Action: action, Resource: resource, Tenant: "public"}); err != nil {
			s.denyAudit(action, resource, err.Error())
			http.Error(w, "capability refused: "+err.Error(), http.StatusForbidden)
			return
		}
		if s.overBudget() { // governed per-minute API budget (C9)
			s.denyAudit(action, resource, "rate budget exceeded")
			http.Error(w, "rate budget exceeded", http.StatusTooManyRequests)
			return
		}
		h(w, r)
	}
}

func (s *Server) denyAudit(action, resource, reason string) {
	if s.Audit != nil {
		s.Audit.Emit(gledger.NewTraceID(), "authz", "deny", gledger.F{"action": action, "resource": resource, "reason": reason})
	}
}

// bearerGrant extracts a capability Grant from the Authorization: Bearer header.
func bearerGrant(r *http.Request) (controlplane.Grant, bool) {
	h := r.Header.Get("Authorization")
	raw, ok := strings.CutPrefix(h, "Bearer ")
	if !ok {
		return controlplane.Grant{}, false
	}
	b, err := base64.URLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return controlplane.Grant{}, false
	}
	var g controlplane.Grant
	if json.Unmarshal(b, &g) != nil {
		return controlplane.Grant{}, false
	}
	return g, true
}

// EncodeToken renders a Grant as the base64url(JSON) bearer token the page and
// API clients present. Used by the host to mint the operator token.
func EncodeToken(g controlplane.Grant) string {
	b, _ := json.Marshal(g)
	return base64.URLEncoding.EncodeToString(b)
}

type scoreRow struct {
	Name      string  `json:"name"`
	Technique string  `json:"technique"`
	Before    float64 `json:"before"`
	After     float64 `json:"after"`
	Pass      bool    `json:"pass"`
}

func asr(target gorauder.Target, seeds []gorauder.Seed) float64 {
	return gorauder.NewRunner(target, gorauder.WithScorer(redteam.Scorer())).Run(context.Background(), seeds).ASR()
}

func (s *Server) scorecard(w http.ResponseWriter, _ *http.Request) {
	var rows []scoreRow
	allPass := true
	for _, c := range redteam.Cases() {
		before := asr(c.Undefended, c.Seeds) * 100
		after := asr(c.Defended, c.Seeds) * 100
		pass := after == 0
		if !pass {
			allPass = false
		}
		rows = append(rows, scoreRow{c.Name, c.Technique, before, after, pass})
	}
	writeJSON(w, map[string]any{"results": rows, "all_pass": allPass})
}

func (s *Server) incidents(w http.ResponseWriter, _ *http.Request) {
	events, _ := ir.Load(s.AuditPath)
	type row struct {
		ID string `json:"id"`
		N  int    `json:"n"`
	}
	var out []row
	for _, id := range ir.Traces(events) {
		out = append(out, row{id, len(ir.Timeline(events, id))})
	}
	writeJSON(w, map[string]any{"traces": out})
}

func (s *Server) incident(w http.ResponseWriter, r *http.Request) {
	trace := r.URL.Query().Get("trace")
	events, _ := ir.Load(s.AuditPath)
	writeJSON(w, map[string]any{"timeline": ir.Timeline(events, trace)})
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Inject the operator capability token so the page's fetch wrapper presents
	// it. base64url has no quote/backslash, so it is safe inside the JS string.
	w.Write([]byte(strings.Replace(ui.Dashboard(), "__CAP_TOKEN__", s.OperatorToken, 1)))
}

// safetyState reports the current kill-switch level.
func (s *Server) safetyState(w http.ResponseWriter, _ *http.Request) {
	lvl := "none"
	allowReq, allowTools := true, true
	if s.Safety != nil {
		lvl = s.Safety.Level().String()
		allowReq, allowTools = s.Safety.AllowRequest(), s.Safety.AllowToolExec()
	}
	writeJSON(w, map[string]any{"level": lvl, "allow_request": allowReq, "allow_tools": allowTools})
}

// killswitch sets the kill level (0 none, 1 block-tools, 2 pause, 3 halt).
func (s *Server) killswitch(w http.ResponseWriter, r *http.Request) {
	if s.Safety == nil {
		http.Error(w, "no kill switch configured", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Level int `json:"level"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Level < 0 || req.Level > 3 {
		http.Error(w, "bad level", http.StatusBadRequest)
		return
	}
	s.Safety.Set("operator", controlplane.KillLevel(req.Level))
	writeJSON(w, map[string]any{"ok": true, "level": s.Safety.Level().String()})
}

// paused reports whether new processing/drops are refused.
func (s *Server) paused() bool { return s.Safety != nil && !s.Safety.AllowRequest() }

// toolsBlocked reports whether side-effect actions (accept/fetch) are refused.
func (s *Server) toolsBlocked() bool { return s.Safety != nil && !s.Safety.AllowToolExec() }

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
