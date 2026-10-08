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
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/t0ul/ai-security-engineering/pkg/agent/extractor"
	"github.com/t0ul/ai-security-engineering/pkg/agent/pipeline"
	"github.com/t0ul/ai-security-engineering/pkg/agent/roster"
	"github.com/t0ul/ai-security-engineering/pkg/agent/tool"
	"github.com/t0ul/ai-security-engineering/pkg/agent/watcher"
	"github.com/t0ul/ai-security-engineering/pkg/controlplane"
	"github.com/t0ul/ai-security-engineering/pkg/datastore"
	"github.com/t0ul/ai-security-engineering/pkg/domain"
	"github.com/t0ul/ai-security-engineering/pkg/durable"
	"github.com/t0ul/ai-security-engineering/pkg/gateway"
	"github.com/t0ul/ai-security-engineering/pkg/netpolicy"
	"github.com/t0ul/ai-security-engineering/pkg/provenance"
	"github.com/t0ul/ai-security-engineering/pkg/rag"
	"github.com/t0ul/ai-security-engineering/pkg/server"
	"github.com/t0ul/gledger"
	"github.com/t0ul/goflage"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8789", "listen address (loopback)")
	drop := flag.String("drop", defaultDrop(), "drop folder (inbox/outbox/processed/logs)")
	allow := flag.String("allow", "schools.nyc.gov,nyc.gov,ps51eliashowe.org,schoolsaccount.nyc", "comma-separated egress allowlist for action/handbook links (parent domains cover subdomains); set empty to deny all")
	vmURL := flag.String("microvm", "http://127.0.0.1:5000", "MicroVM vsock bridge for in-sandbox fetches")
	gwFlag := flag.String("gateway", envOr("GATEWAY_URL", "http://127.0.0.1:4000/v1/chat/completions"), "gouncer gateway chat-completions URL (the single source for the chat + health-check endpoint)")
	flag.Parse()

	// Single source for the gateway endpoint (was two hardcoded literals). The chat
	// path posts here; the health check dials the host:port parsed from it.
	dial := "127.0.0.1:4000"
	if u, err := url.Parse(*gwFlag); err == nil && u.Host != "" {
		dial = u.Host
	}
	gw = gateway.New(*gwFlag, dial)

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
		opToken = server.EncodeToken(g)
	}
	corpusPath := filepath.Join(*drop, "corpus.db")
	// Declared early so the chat closure below can capture them; assigned once the
	// governed planes are built further down (consts are the fail-closed default).
	var prompts *controlplane.Prompts
	var sampling *controlplane.Sampling
	var inv *datastore.Store
	var search func(string, int) ([]domain.SearchHit, error)
	var chat func(string, bool, string) (string, []string, error)
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
		search = func(q string, k int) ([]domain.SearchHit, error) {
			if _, err := authz.Verify(readerGrant, controlplane.Capability{Action: controlplane.ActionList, Resource: "corpus", Tenant: "public"}); err != nil {
				return nil, fmt.Errorf("rag-reader capability refused: %w", err)
			}
			chunks, err := reader.Query("public", q, k)
			if err != nil {
				return nil, err
			}
			out := make([]domain.SearchHit, 0, len(chunks))
			for _, c := range chunks {
				out = append(out, domain.SearchHit{Source: c.DocID, Snippet: server.Snippet(c.Text), Untrusted: c.Prov == rag.Untrusted})
			}
			return out, nil
		}
		// Chat: the conversational front-end over the corpus. DEFENDED uses
		// rag.Assemble (XML-encapsulate + injection-neutralize untrusted chunks, M8);
		// UNSAFE (demo) raw-concats them, so a poisoned doc's injection reaches the
		// model as if trusted — a live ADD demo (ChatRAGInjection). Same reader NHI
		// grant as search, so Halt / residency gate it too.
		chat = func(question string, unsafe bool, appData string) (string, []string, error) {
			if _, err := authz.Verify(readerGrant, controlplane.Capability{Action: controlplane.ActionList, Resource: "corpus", Tenant: "public"}); err != nil {
				return "", nil, fmt.Errorf("rag-reader capability refused: %w", err)
			}
			// Trusted context: the host clock is the ONLY authority for dates. It is
			// never sourced from the corpus, so a poisoned email ("today is …") cannot
			// move the agent's clock (the trust boundary the chat_system prompt enforces).
			dateBlock := gateway.TrustedDateBlock(time.Now())
			chunks, err := reader.Query("public", question, 5)
			if err != nil {
				return "", nil, err
			}
			sources := make([]string, 0, len(chunks))
			for _, c := range chunks {
				sources = append(sources, c.DocID)
			}
			var context string
			if unsafe {
				var b strings.Builder
				for _, c := range chunks {
					b.WriteString(c.Text)
					b.WriteString("\n")
				}
				context = b.String() // CONTROLS OFF: raw untrusted text, no encapsulation/scrub
			} else {
				context = rag.Assemble(chunks) // encapsulated + injection-neutralized (M8)
			}
			// chat_system is a GOVERNED prompt (C2), decoding is GOVERNED sampling (C5),
			// and the model binding is DB config — all resolved live, none hardcoded.
			return gw.ChatAnswer(chatParams(sampling, inv), prompts.Text("chat_system"), dateBlock, appData, context, question, unsafe), sources, nil
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
	mcpReg := mcpRegistry{gw: toolGW, safety: safety}

	// Governed prompts (C2): the console lists/activates/rolls-back the system
	// prompts the LLM planner/coder/extractor resolve at runtime. Activations are
	// versioned, hashed, audited to gledger, and persisted to the cpstore
	// inventory so they survive restarts and are attributable.
	promptDefaults := map[string]string{
		"planner":     controlplane.PlannerSystemPrompt,
		"coder":       controlplane.CoderSystemPrompt,
		"extractor":   extractor.ExtractionPrompt,
		"chat_system": controlplane.ChatSystemPrompt,
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
		"chat":      {Temperature: 0.2, MaxTokens: 400}, // governed chat decoding (C5), was hardcoded in the gateway
	}
	// Governed budgets (C9): the "api" rate limit is enforced in-app; model
	// rate/token/spend are gateway-side. KeyRef is a POINTER (env-var name), never
	// the key value — secrets stay out of the config/DB.
	budgetDefaults := map[string]controlplane.BudgetConfig{
		"api":     {RatePerMin: 0}, // 0 = unlimited until the operator sets one
		"gateway": {MaxTokens: 900, KeyRef: "GATEWAY_KEY"},
	}
	var policies *controlplane.Policies
	var budgets *controlplane.Budgets
	if db, ierr := datastore.Open(filepath.Join(*drop, "inventory.db")); ierr == nil {
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
	var profileSvc server.ProfileStore
	if inv != nil {
		for _, n := range []string{"planner", "coder", "extractor", "chat_system"} {
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
		profileSvc = profileStore{inv: inv}
	}

	// Data-flywheel: operator accept/reject decisions are durable ground-truth.
	var flywheelSvc server.Flywheel
	if inv != nil {
		flywheelSvc = flywheel{inv: inv, audit: audit}
	}

	// Known-good bundles (C10): snapshot the whole governed plane under a label and
	// roll it all back in one step. Needs the DB.
	var bundleSvc server.BundleStore
	if inv != nil {
		bundleSvc = bundleStore{inv: inv, audit: audit, prompts: prompts, sampling: sampling, policies: policies}
	}

	// Eval surfaces (C7): the Eval card + a "Test" button that shadow-evals a
	// candidate extractor prompt before activation. The extraction mode is now
	// passed explicitly (eval.ScoreWithMode "llm"), not via the process-global
	// EXTRACT_MODE env; the remaining shared state is the extractor's prompt
	// override, so these still serialize on evalMu. Live runs need the gouncer
	// gateway (cmd/livecheck / a running modeld+gouncer); without it the extractor
	// falls back to regex and we say so.
	evalSvc := &evalService{inv: inv, gw: gw, prompts: prompts,
		labels: []string{"testdata/emaildrop/labels/3.json", "testdata/emaildrop/labels/1.json"}}

	srv := &http.Server{
		Addr: *addr,
		Handler: server.New(server.Config{
			AuditPath: auditPath, OutboxDir: cfg.Outbox, InboxPath: cfg.Inbox,
			Egress: egress, Fetch: fetch, Verifier: verifier, Safety: safety, Search: search, Index: enrichIndex,
			Flywheel: flywheelSvc, Chat: chat,
			Authz: authz, OperatorToken: opToken, Audit: audit,
			MCP: mcpReg, Prompts: prompts, Policies: policies, Sampling: sampling, Budgets: budgets,
			Eval:    evalSvc,
			Bundles: bundleSvc,
			Profile: profileSvc,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second, // live eval/chat can be slow
		IdleTimeout:       120 * time.Second,
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

// gw is the chat gateway client (pkg/gateway), set from the -gateway flag in main.
// The chat path and the eval/health checks go through it. It starts at the
// fail-closed default so it is never nil before flag parsing.
var gw = gateway.New("http://127.0.0.1:4000/v1/chat/completions", "127.0.0.1:4000")

// envOr returns the env var value or a fallback.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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

// chatParams resolves the chat model + decoding from governed config at call time
// (C5 sampling "chat" + the chat_model DB key), with the shipped consts as the
// fail-closed default — nothing hardcoded in the gateway (DB-first config).
func chatParams(sampling *controlplane.Sampling, inv *datastore.Store) gateway.ChatParams {
	p := gateway.ChatParams{Model: "planner", Temperature: 0.2, MaxTokens: 400}
	if sampling != nil {
		if sc := sampling.Config("chat"); sc.MaxTokens > 0 {
			p.Temperature, p.MaxTokens = sc.Temperature, sc.MaxTokens
		}
	}
	if inv != nil {
		if m, ok, _ := inv.GetConfig("chat_model"); ok && m != "" {
			p.Model = m
		}
	}
	return p
}
