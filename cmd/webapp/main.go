// Command webapp is the all-in-one local agent app: it creates a drop folder
// (on your Desktop by default), watches it for dropped .txt emails and turns them
// into signed .ics events, and serves the operator console — Calendar, Security
// (run the scorecard), and Incidents (replay). One binary, loopback only.
//
//	webapp [-addr 127.0.0.1:8788] [-drop ~/Desktop/Email-to-Calendar]
//
// Drop a .txt email into <drop>/inbox and it appears on the Calendar tab.
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/t0ul/ai-security-engineering/agent/eval"
	"github.com/t0ul/ai-security-engineering/agent/extractor"
	"github.com/t0ul/ai-security-engineering/agent/pipeline"
	"github.com/t0ul/ai-security-engineering/agent/roster"
	"github.com/t0ul/ai-security-engineering/agent/tool"
	"github.com/t0ul/ai-security-engineering/agent/watcher"
	"github.com/t0ul/ai-security-engineering/controlplane"
	"github.com/t0ul/ai-security-engineering/cpstore"
	"github.com/t0ul/ai-security-engineering/durable"
	"github.com/t0ul/ai-security-engineering/netpolicy"
	"github.com/t0ul/ai-security-engineering/provenance"
	"github.com/t0ul/ai-security-engineering/rag"
	"github.com/t0ul/ai-security-engineering/redteam"
	"github.com/t0ul/ai-security-engineering/webapp"
	"github.com/t0ul/gledger"
	"github.com/t0ul/goflage"
	"github.com/t0ul/gorauder"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8789", "listen address (loopback)")
	drop := flag.String("drop", defaultDrop(), "drop folder (inbox/outbox/processed/logs)")
	allow := flag.String("allow", "schools.nyc.gov,nyc.gov,ps51eliashowe.org,schoolsaccount.nyc", "comma-separated egress allowlist for action/handbook links (parent domains cover subdomains); set empty to deny all")
	vmURL := flag.String("microvm", "http://127.0.0.1:5000", "MicroVM vsock bridge for in-sandbox fetches")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := watcher.NewConfig(*drop)
	auditPath := filepath.Join(cfg.Logs, "audit.jsonl")
	audit, err := gledger.Open(auditPath, "watcher")
	if err != nil {
		log.Fatalf("webapp: %v", err)
	}

	reg := tool.NewRegistry()
	roster.Register(reg)
	pipe := &pipeline.Pipeline{Registry: reg, Audit: audit, OutboxDir: cfg.Outbox, DefaultYear: time.Now().Year(), ToolNames: roster.Names()}

	// Provenance (M20): a persistent agent identity signs every emitted .ics, so
	// the UI can prove the agent produced it unaltered. The ed25519 seed lives in
	// the drop dir (0600); generated once, reused across restarts.
	signer, verifier := agentIdentity(filepath.Join(*drop, "agent.key"))
	pipe.Signer = signer

	// Kill switch (M18): Pause halts processing of new drops; the console toggles it.
	safety := controlplane.NewSafety(audit)
	pipe.Halted = func() bool { return !safety.AllowRequest() }

	// Non-human identity + capability authZ (C4b): the console becomes an authZ'd
	// API. Every mutating/listing endpoint verifies a capability Grant signed by
	// the agent key. One broad operator Grant drives the UI; engaging Halt revokes
	// every grant (Authority.Halted), so in-flight side effects die — not just new
	// drops paused. Tools/agent get narrower grants as those paths are wired (C4d/e).
	authz := controlplane.NewAuthority(signer, verifier)
	authz.Halted = func() bool { return safety.Level() >= controlplane.LevelHalt }
	// Data-residency policy (C4e): a subject bound to an off-host (frontier) model
	// may not be granted list/export of confidential data. Today every subject is
	// on-host (frontier set empty), so nothing is refused — but the rule is live
	// and swap-ready: binding a subject to a frontier model (a governed C5 swap)
	// refuses its next issuance. The corpus is NOT confidential here because
	// goflage scrubs it on ingest (declassified); contacts/email bodies are.
	authz.IssuePolicy = controlplane.ResidencyPolicy(
		map[string]bool{}, // frontier-bound subjects (none yet)
		map[string]bool{"contacts": true, "email-bodies": true},
	)
	var opToken string
	if g, gerr := authz.Issue(controlplane.Capability{Subject: "operator", Action: controlplane.Scope, Resource: controlplane.Scope, Tenant: controlplane.Scope}, 30*24*time.Hour); gerr == nil {
		opToken = webapp.EncodeToken(g)
	}
	corpusPath := filepath.Join(*drop, "corpus.db")
	var search func(string, int) ([]webapp.SearchHit, error)
	var enrichIndex func(source, text string) error
	if corpus, cerr := rag.Open(corpusPath); cerr == nil {
		defer corpus.Close()
		pipe.Index = func(traceID, source, rawText string) error {
			return corpus.Add(rag.Doc{ID: source, Text: rawText, Prov: rag.Untrusted}) // readable source in Ask results
		}
		// R6 link enrichment: fetched handbook bytes are untrusted and may carry PII
		// — scrub on ingest (M3), then index as Untrusted so Ask can use but never
		// trust them.
		enrichIndex = func(source, text string) error {
			scrubbed, _ := goflage.New().Scrub(text)
			return corpus.Add(rag.Doc{ID: source, Text: scrubbed, Prov: rag.Untrusted})
		}
		// Retrieval runs on a SEPARATE read-only handle (C4d least privilege): Ask
		// can query but the engine refuses any write, so a bug or injection on the
		// read path cannot mutate or poison the corpus. Falls back to the writable
		// handle only if the read-only open fails.
		reader := corpus
		if ro, rerr := rag.OpenReadOnly(corpusPath); rerr == nil {
			defer ro.Close()
			reader = ro
		}
		// The RAG reader is its own non-human identity (C4e): a grant scoped to
		// list/corpus, minted through the residency policy. Verifying it per query
		// means Halt revokes retrieval too, and a frontier-bound reader would be
		// refused at issuance (fail-closed: no grant -> no search).
		readerGrant, _ := authz.Issue(controlplane.Capability{Subject: "rag-reader", Action: controlplane.ActionList, Resource: "corpus", Tenant: "public"}, 30*24*time.Hour)
		// Ask-School: lexical search over the scrubbed corpus (M8 — recalled text
		// is untrusted data). "public" tenant: this is a single-household app.
		search = func(q string, k int) ([]webapp.SearchHit, error) {
			if _, err := authz.Verify(readerGrant, controlplane.Capability{Action: controlplane.ActionList, Resource: "corpus", Tenant: "public"}); err != nil {
				return nil, fmt.Errorf("rag-reader capability refused: %w", err)
			}
			chunks, err := reader.Query("public", q, k)
			if err != nil {
				return nil, err
			}
			out := make([]webapp.SearchHit, 0, len(chunks))
			for _, c := range chunks {
				out = append(out, webapp.SearchHit{Source: c.DocID, Snippet: webapp.Snippet(c.Text), Untrusted: c.Prov == rag.Untrusted})
			}
			return out, nil
		}
	}

	w := &watcher.Watcher{Pipe: pipe, Cfg: cfg}
	if err := w.EnsureDirs(); err != nil {
		log.Fatalf("webapp: %v", err)
	}
	go func() {
		if err := w.Watch(ctx, func(s string) { log.Println("[watcher]", s) }); err != nil {
			log.Println("[watcher] stopped:", err)
		}
	}()

	// Action-link egress: deny-by-default allowlist, and an in-MicroVM fetcher so
	// a link from an untrusted email/handbook executes in the sandbox, not on the
	// host. Without -allow, action links are refused.
	var allowHosts []string
	for _, h := range strings.Split(*allow, ",") {
		if h = strings.TrimSpace(h); h != "" {
			allowHosts = append(allowHosts, h)
		}
	}
	egress := netpolicy.Policy{Allow: allowHosts}
	interp := controlplane.NewInterpreter(*vmURL, audit)
	fetchTool := controlplane.SandboxFetchTool(interp)
	// MCP on the app path (C4e): the executing tool is reached through a gustoms
	// gateway (manifest-pinned, allow-listed), not called directly — so a swapped
	// tool is rejected, and the authorizer denies every call while the kill switch
	// blocks tools (the functional "drop pins on halt").
	toolGW := controlplane.NewToolGateway("sandbox", fetchTool, safety.AllowToolExec)
	// The action-fetcher is its own NHI: a grant scoped to export/link (egress).
	// Verifying it means Halt revokes the fetch in-flight, on top of the operator's
	// HTTP gate — the human delegates to a least-privilege machine identity.
	fetcherGrant, _ := authz.Issue(controlplane.Capability{Subject: "action-fetcher", Action: controlplane.ActionExport, Resource: "link", Tenant: "public"}, 30*24*time.Hour)

	// Durable execution + exactly-once (reliability track): the action-link fetch
	// is the app's side-effecting tool, so route it through a hash-chained step
	// log. A duplicated/replayed action with the same key returns the first result
	// instead of re-firing (exactly-once); the log is chain-verified on open, so a
	// tampered checkpoint fails closed — we start fresh rather than resume into a
	// forged state.
	steps, serr := durable.Open(filepath.Join(*drop, "steps.jsonl"))
	if serr != nil {
		log.Printf("[durable] step log failed verification (%v) — starting fresh, NOT resuming into it", serr)
		audit.Emit(gledger.NewTraceID(), "durable", "tamper_detected", gledger.F{"error": serr.Error()})
		steps, _ = durable.Open("")
	}
	fetch := func(ctx context.Context, url string) (string, error) {
		if _, err := authz.Verify(fetcherGrant, controlplane.Capability{Action: controlplane.ActionExport, Resource: "link", Tenant: "public"}); err != nil {
			return "", fmt.Errorf("action-fetcher capability refused: %w", err)
		}
		out, _, derr := steps.Do("webapp", "tool", "fetch:"+url, func() (string, error) {
			o, err := toolGW.Call(ctx, gledger.NewTraceID(), "action-fetcher", "sandbox", "web_fetch", map[string]any{"url": url, "trace_id": gledger.NewTraceID()})
			if err != nil {
				return "", err
			}
			s, _ := o.(map[string]any)["output"].(string)
			return s, nil
		})
		return out, derr
	}

	// MCP governance tab (C6): surface the gateway's servers, their advertised +
	// allow-listed tools, the approved pin vs the live manifest (rug-pull alert),
	// and an operator Approve (re-pin). "blocked" reflects the kill switch.
	mcpList := func() []webapp.MCPServer {
		var out []webapp.MCPServer
		for _, st := range toolGW.Status(context.Background()) {
			status := "pinned"
			switch {
			case st.Err != "":
				status = "error"
			case !safety.AllowToolExec():
				status = "blocked"
			case st.Pinned == "":
				status = "unapproved"
			case st.Mismatch:
				status = "rug-pull"
			}
			out = append(out, webapp.MCPServer{
				Name: st.Name, Tools: st.Tools, Allowed: st.Allowed,
				Pinned: shortHash(st.Pinned), Current: shortHash(st.Current), Status: status,
			})
		}
		return out
	}
	mcpApprove := func(server string) error {
		return toolGW.Approve(context.Background(), gledger.NewTraceID(), server)
	}

	// Governed prompts (C2): the console lists/activates/rolls-back the system
	// prompts the LLM planner/coder/extractor resolve at runtime. Activations are
	// versioned, hashed, audited to gledger, and persisted to the cpstore
	// inventory so they survive restarts and are attributable.
	promptDefaults := map[string]string{
		"planner":   controlplane.PlannerSystemPrompt,
		"coder":     controlplane.CoderSystemPrompt,
		"extractor": extractor.ExtractionPrompt,
	}
	// Governed policy allowlists (C8): egress hosts + exec argv[0]s as versioned,
	// hashed, rollback-able artifacts. The action egress check resolves its
	// allowlist from here live; deny-by-default (private/IMDS, argcheck) is
	// unaffected by a widening.
	policyDefaults := map[string][]string{
		"egress": allowHosts,
		"exec":   controlplane.DefaultAllowedCommands,
	}
	// Governed sampling (C5): decoding params as versioned artifacts; the extractor
	// LLM path reads them. seed>0 → reproducible generations.
	samplingDefaults := map[string]controlplane.SamplingConfig{
		"extractor": {Temperature: 0.1, MaxTokens: 900},
	}
	// Governed budgets (C9): the "api" rate limit is enforced in-app; model
	// rate/token/spend are gateway-side. KeyRef is a POINTER (env-var name), never
	// the key value — secrets stay out of the config/DB.
	budgetDefaults := map[string]controlplane.BudgetConfig{
		"api":     {RatePerMin: 0}, // 0 = unlimited until the operator sets one
		"gateway": {MaxTokens: 900, KeyRef: "GATEWAY_KEY"},
	}
	var prompts *controlplane.Prompts
	var policies *controlplane.Policies
	var sampling *controlplane.Sampling
	var budgets *controlplane.Budgets
	var inv *cpstore.Store
	if db, ierr := cpstore.Open(filepath.Join(*drop, "inventory.db")); ierr == nil {
		inv = db
		defer inv.Close()
		prompts = controlplane.GovernedPrompts(promptDefaults, inv, audit)
		policies = controlplane.GovernedPolicies(policyDefaults, inv, audit)
		sampling = controlplane.GovernedSampling(samplingDefaults, inv, audit)
		budgets = controlplane.GovernedBudgets(budgetDefaults, inv, audit)
	} else {
		prompts = controlplane.NewPrompts(promptDefaults)
		policies = controlplane.NewPolicies(policyDefaults)
		sampling = controlplane.NewSampling(samplingDefaults)
		budgets = controlplane.NewBudgets(budgetDefaults)
	}
	// Activating the "extractor" sampling drives the live LLM decoding params.
	baseSampOnActivate := sampling.OnActivate
	sampling.OnActivate = func(sv controlplane.SamplingVersion) {
		if baseSampOnActivate != nil {
			baseSampOnActivate(sv)
		}
		if sv.Name == "extractor" {
			extractor.SetSampling(sv.Config.Temperature, sv.Config.MaxTokens, sv.Config.Seed)
		}
	}
	// Activating the "extractor" prompt actually drives the LLM extractor (C5 down
	// payment): chain the governed OnActivate to set the live extraction prompt.
	baseOnActivate := prompts.OnActivate
	prompts.OnActivate = func(pv controlplane.PromptVersion) {
		if baseOnActivate != nil {
			baseOnActivate(pv)
		}
		if pv.Name == "extractor" {
			extractor.SetExtractionPrompt(pv.Text)
		}
	}

	// DB is the source of truth (not consts/files): rehydrate the active prompts,
	// policies, and the child profile persisted by a prior session from cpstore on
	// boot. The shipped consts remain only the fail-closed default when the DB has
	// no row for a knob.
	var profileLoad func() webapp.Profile
	var profileSave func(webapp.Profile) error
	if inv != nil {
		for _, n := range []string{"planner", "coder", "extractor"} {
			if text, ok, _ := inv.LatestPromptText(n); ok {
				prompts.Rehydrate(n, text)
			}
		}
		for _, n := range []string{"egress", "exec"} {
			if items, ok, _ := inv.LatestPolicyItems(n); ok {
				policies.Rehydrate(n, items)
			}
		}
		if cfgJSON, ok, _ := inv.LatestSampling("extractor"); ok {
			var cfg controlplane.SamplingConfig
			if json.Unmarshal([]byte(cfgJSON), &cfg) == nil {
				sampling.Rehydrate("extractor", cfg)
			}
		}
		for _, n := range []string{"api", "gateway"} {
			if cfgJSON, ok, _ := inv.LatestBudget(n); ok {
				var cfg controlplane.BudgetConfig
				if json.Unmarshal([]byte(cfgJSON), &cfg) == nil {
					budgets.Rehydrate(n, cfg)
				}
			}
		}
		extractor.SetExtractionPrompt(prompts.Text("extractor"))
		sc := sampling.Config("extractor")
		extractor.SetSampling(sc.Temperature, sc.MaxTokens, sc.Seed)
		// Logical extractor model binding + grammar JSON (C5), from governed config
		// (DB is the source of truth). Defaults: model "planner", grammar off.
		if m, ok, _ := inv.GetConfig("extractor_model"); ok && m != "" {
			extractor.SetModel(m)
		}
		if j, ok, _ := inv.GetConfig("extractor_json"); ok && j == "true" {
			extractor.SetJSONMode(true)
		}
		profileLoad = func() webapp.Profile {
			var p webapp.Profile
			if v, ok, _ := inv.GetConfig("profile"); ok {
				_ = json.Unmarshal([]byte(v), &p)
			}
			return p
		}
		profileSave = func(p webapp.Profile) error {
			raw, _ := json.Marshal(p)
			return inv.SetConfig("profile", string(raw))
		}
	}

	// Data-flywheel: operator accept/reject decisions are durable ground-truth.
	var feedback func(decision, source, title string)
	var flywheelStats func() (int, int)
	if inv != nil {
		feedback = func(decision, source, title string) {
			_ = inv.RecordFeedback(source, title, decision)
			audit.Emit(gledger.NewTraceID(), "feedback", decision, gledger.F{"source": source, "title": title})
		}
		flywheelStats = func() (int, int) {
			a, r, _ := inv.FeedbackStats()
			return a, r
		}
	}

	// Known-good bundles (C10): snapshot the whole governed plane under a label and
	// roll it all back in one step. Needs the DB.
	var bundleList func() []webapp.BundleInfo
	var bundleSave func(string) error
	var bundleApply func(string) error
	if inv != nil {
		bundleList = func() []webapp.BundleInfo {
			rows, _ := inv.ListBundles(50)
			out := make([]webapp.BundleInfo, 0, len(rows))
			for _, b := range rows {
				out = append(out, webapp.BundleInfo{Label: b.Label, At: b.At.Format("2006-01-02 15:04")})
			}
			return out
		}
		bundleSave = func(label string) error {
			b := controlplane.Snapshot(label, prompts, sampling, policies)
			raw, _ := json.Marshal(b)
			audit.Emit(gledger.NewTraceID(), "bundle", "saved", gledger.F{"label": label})
			return inv.SaveBundle(label, string(raw))
		}
		bundleApply = func(label string) error {
			cfg, ok, err := inv.GetBundle(label)
			if err != nil || !ok {
				return fmt.Errorf("no such snapshot %q", label)
			}
			var b controlplane.Bundle
			if err := json.Unmarshal([]byte(cfg), &b); err != nil {
				return err
			}
			b.Apply(prompts, sampling, policies)
			audit.Emit(gledger.NewTraceID(), "bundle", "rolled_back", gledger.F{"label": label})
			return nil
		}
	}

	// Eval surfaces (C7): the Eval card + a "Test" button that shadow-evals a
	// candidate extractor prompt before activation. Live eval and shadow eval
	// mutate process-global state (EXTRACT_MODE, the extractor prompt override), so
	// serialize them. Live runs need the gouncer gateway at :4000 (cmd/livecheck /
	// a running modeld+gouncer); without it the extractor falls back to regex and
	// we say so.
	var evalMu sync.Mutex
	labels := []string{"testdata/emaildrop/labels/3.json", "testdata/emaildrop/labels/1.json"}
	evalHistory := func() []webapp.EvalResult {
		if inv == nil {
			return nil
		}
		rows, _ := inv.ListEvals(20)
		out := make([]webapp.EvalResult, 0, len(rows))
		for _, e := range rows {
			out = append(out, webapp.EvalResult{Label: e.Label, F1: e.F1, At: e.At.Format("2006-01-02 15:04")})
		}
		return out
	}
	evalRun := func() ([]webapp.EvalResult, string) {
		evalMu.Lock()
		defer evalMu.Unlock()
		live := gatewayUp()
		prev := os.Getenv("EXTRACT_MODE")
		os.Setenv("EXTRACT_MODE", "llm")
		defer os.Setenv("EXTRACT_MODE", prev)
		var out []webapp.EvalResult
		for _, l := range labels {
			rep, err := eval.Score(l, true)
			if err != nil {
				continue
			}
			if inv != nil {
				_ = inv.RecordEval(rep.Source, rep.F1)
			}
			out = append(out, webapp.EvalResult{Label: rep.Source, F1: rep.F1})
		}
		return out, evalMode(live)
	}
	promptTest := func(name, candidate string) webapp.PromptTestResult {
		if name != "extractor" {
			return webapp.PromptTestResult{Note: "shadow eval applies to the 'extractor' prompt (the labeled eval path); planner/coder drive the orchestrator demo, not this eval."}
		}
		evalMu.Lock()
		defer evalMu.Unlock()
		live := gatewayUp()
		prevMode := os.Getenv("EXTRACT_MODE")
		os.Setenv("EXTRACT_MODE", "llm")
		defer os.Setenv("EXTRACT_MODE", prevMode)
		extractor.SetExtractionPrompt(candidate)
		defer extractor.SetExtractionPrompt(prompts.Text("extractor")) // restore the governed-active prompt

		rep, err := eval.Score("testdata/emaildrop/labels/1.json", true)
		f1 := 0.0
		if err == nil {
			f1 = rep.F1
		}
		baseline := 0.0
		if inv != nil {
			if e, ok, _ := inv.LatestEval("samples/1.txt"); ok {
				baseline = e.F1
			}
		}
		asrPass := true
		for _, c := range redteam.Cases() {
			if defendedASR(c) > 0 {
				asrPass = false
				break
			}
		}
		asrVal := 0.0
		if !asrPass {
			asrVal = 1.0
		}
		gateOK, _ := controlplane.PromotionGate{MinF1: baseline, F1: func() float64 { return f1 }, ASR: func() float64 { return asrVal }}.Allow()
		return webapp.PromptTestResult{F1: f1, Baseline: baseline, ASRPass: asrPass, GateOK: gateOK, Mode: evalMode(live)}
	}

	srv := &http.Server{
		Addr: *addr,
		Handler: (&webapp.Server{
			AuditPath: auditPath, OutboxDir: cfg.Outbox, InboxPath: cfg.Inbox,
			Egress: egress, Fetch: fetch, Verifier: verifier, Safety: safety, Search: search, Index: enrichIndex,
			Feedback: feedback, FlywheelStats: flywheelStats,
			Authz: authz, OperatorToken: opToken, Audit: audit,
			MCP: mcpList, MCPApprove: mcpApprove, Prompts: prompts, Policies: policies, Sampling: sampling, Budgets: budgets,
			EvalHistory: evalHistory, EvalRun: evalRun, PromptTest: promptTest,
			BundleList: bundleList, BundleSave: bundleSave, BundleApply: bundleApply,
			ProfileLoad: profileLoad, ProfileSave: profileSave,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() { <-ctx.Done(); srv.Close() }()

	fmt.Printf("agent console: http://%s\n", *addr)
	fmt.Printf("drop .txt emails into: %s\n", cfg.Inbox)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("webapp: %v", err)
	}
}

// agentIdentity loads (or creates) the persistent ed25519 seed at path and
// returns the signer + a verifier trusting that key. On any error it falls back
// to an ephemeral key so the app still runs (signatures just won't verify across
// restarts).
func agentIdentity(path string) (*provenance.Signer, *provenance.Verifier) {
	const keyID = "email-agent"
	seed, err := os.ReadFile(path)
	if err != nil || len(seed) != ed25519.SeedSize {
		seed = make([]byte, ed25519.SeedSize)
		if _, rerr := rand.Read(seed); rerr == nil {
			_ = os.WriteFile(path, seed, 0o600)
		}
	}
	signer, pub, serr := provenance.SignerFromSeed(keyID, seed)
	if serr != nil {
		signer, pub, _ = provenance.NewSigner(keyID) // ephemeral fallback
	}
	return signer, provenance.NewVerifier().Trust(keyID, pub)
}

// gatewayUp reports whether the gouncer gateway is reachable, so a live eval run
// can say whether it ran against the model or fell back to regex.
func gatewayUp() bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:4000", 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func evalMode(live bool) string {
	if live {
		return "llm (gouncer :4000)"
	}
	return "gateway :4000 down → regex fallback; start cmd/livecheck or modeld+gouncer for the live path"
}

// defendedASR runs one ADD case's defended target and returns its attack success
// rate (0 = the control holds).
func defendedASR(c redteam.Case) float64 {
	return gorauder.NewRunner(c.Defended, gorauder.WithScorer(redteam.Scorer())).Run(context.Background(), c.Seeds).ASR()
}

// shortHash trims a hex manifest hash to a display prefix.
func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

// defaultDrop is ~/Desktop/Email-to-Calendar (Mac-friendly), falling back to the
// working directory if the home/Desktop can't be resolved.
func defaultDrop() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "Email-to-Calendar"
	}
	return filepath.Join(home, "Desktop", "Email-to-Calendar")
}
