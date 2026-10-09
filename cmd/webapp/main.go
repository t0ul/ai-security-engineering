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
	"bufio"
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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/t0ul/ai-security-engineering/internal/modelcatalog"
	"github.com/t0ul/ai-security-engineering/internal/modelstack"
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
	// Infra/deploy config is 12-factor: a .env (gitignored) seeds the environment,
	// each flag defaults to its env var, and an explicit CLI flag overrides. Real
	// env vars win over .env. Business/behavior config lives in the DB, not here.
	loadDotenv(".env")
	addr := flag.String("addr", envOr("WEBAPP_ADDR", "127.0.0.1:8789"), "listen address (loopback)")
	drop := flag.String("drop", envOr("WEBAPP_DROP", defaultDrop()), "drop folder (inbox/outbox/processed/logs); the SQLite DBs live here")
	allow := flag.String("allow", envOr("WEBAPP_ALLOW", "schools.nyc.gov,nyc.gov,ps51eliashowe.org,schoolsaccount.nyc"), "comma-separated egress allowlist for action/handbook links (parent domains cover subdomains); set empty to deny all")
	vmURL := flag.String("microvm", envOr("MICROVM_URL", "http://127.0.0.1:5000"), "MicroVM vsock bridge for in-sandbox fetches")
	gwFlag := flag.String("gateway", envOr("GATEWAY_URL", "http://127.0.0.1:4000/v1/chat/completions"), "gouncer gateway chat-completions URL (the single source for the chat + health-check endpoint)")
	assetsDir := flag.String("assets", envOr("WEBAPP_ASSETS", "set-up/vm-assets"), "directory holding the served GGUF models (for the Runtime health panel)")
	seedDir := flag.String("seed", envOr("WEBAPP_SEED", "examples"), "dir of example emails to ingest into the inbox on first run (empty to disable)")
	autoModels := flag.Bool("automodels", envOr("WEBAPP_AUTOMODELS", "true") != "false", "automatically bring the local models up on startup (planner/coder + gateway) so chat & extraction work out of the box")
	allowEphemeralID := flag.Bool("allow-ephemeral-identity", envOr("WEBAPP_ALLOW_EPHEMERAL_IDENTITY", "false") == "true", "allow starting on a throwaway in-memory signing key when the persistent agent seed cannot be loaded/persisted (DEV ONLY: signatures will not verify across restarts)")
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

	// Shutdown cleanups (model servers, embed server). Registered as they start and
	// run SYNCHRONOUSLY on shutdown (LIFO) — NOT in a goroutine that races the process
	// exit — so spawned llama.cpp processes are actually killed before we exit, not
	// orphaned. Thread-safe: model auto-start runs in a background goroutine.
	var cleanupMu sync.Mutex
	var cleanups []func()
	addCleanup := func(f func()) {
		cleanupMu.Lock()
		cleanups = append(cleanups, f)
		cleanupMu.Unlock()
	}

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
	keyPath := filepath.Join(*drop, "agent.key")
	signer, verifier, persistentID, err := agentIdentity(keyPath)
	if err != nil {
		log.Fatalf("webapp: agent identity: %v", err)
	}
	if !persistentID {
		// Fail closed: an ephemeral identity means every .ics signed this run stops
		// verifying after a restart, and the authz trust anchor changes — a silent
		// security downgrade. Refuse to start unless a dev explicitly opts in.
		if !*allowEphemeralID {
			log.Fatalf("webapp: SECURITY: could not load or persist the agent identity at %s — refusing to start on an ephemeral key (signatures would not verify across restarts). Fix the seed path/permissions, or pass -allow-ephemeral-identity (env WEBAPP_ALLOW_EPHEMERAL_IDENTITY=true) for a throwaway dev run.", keyPath)
		}
		log.Printf("webapp: WARNING: running on an EPHEMERAL agent identity (seed at %s not persisted) — previously signed .ics will NOT verify and this key dies on restart. Dev mode only.", keyPath)
	}
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
	// The App surface runs on a NARROW, least-privilege grant (C4b): it covers only
	// the household consumer scopes — list the corpus, accept/reject calendar items,
	// drop emails, save the profile, follow an action link — and NOTHING on the
	// governed control plane. A multi-scope signed grant (primary + extra) expresses
	// this; the operator's wildcard grant still drives Studio. So the App page is
	// cryptographically unable to reach a governance endpoint, not merely UI-hidden.
	var appToken string
	if g, gerr := authz.IssueScoped(
		controlplane.Capability{Subject: "app", Action: controlplane.ActionList, Resource: "corpus", Tenant: "public"},
		[]controlplane.GrantScope{
			{Action: controlplane.ActionWrite, Resource: "calendar", Tenant: "public"},
			{Action: controlplane.ActionWrite, Resource: "inbox", Tenant: "public"},
			{Action: controlplane.ActionWrite, Resource: "profile", Tenant: "public"},
			{Action: controlplane.ActionExport, Resource: "link", Tenant: "public"},
		}, 30*24*time.Hour); gerr == nil {
		appToken = server.EncodeToken(g)
	}
	corpusPath := filepath.Join(*drop, "corpus.db")
	// Declared early so the chat closure below can capture them; assigned once the
	// governed planes are built further down (consts are the fail-closed default).
	var prompts *controlplane.Prompts
	var sampling *controlplane.Sampling
	var inv *datastore.Store
	var reader *rag.Store              // RAG read handle; nil if the corpus failed to open
	var corpusWritable *rag.Store      // writable corpus handle (for the RAG lab reindex)
	var ragSemantic atomic.Bool        // true = chat uses semantic (vector) retrieval; shared with the RAG lab
	var readerGrant controlplane.Grant // the rag-reader NHI grant (C4e)
	var search func(string, int) ([]domain.SearchHit, error)
	var enrichIndex func(source, text string) error
	if corpus, cerr := rag.Open(corpusPath); cerr == nil {
		defer corpus.Close()
		corpusWritable = corpus
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
		reader = corpus
		if ro, rerr := rag.OpenReadOnly(corpusPath); rerr == nil {
			defer ro.Close()
			reader = ro
		}
		// The RAG reader is its own non-human identity (C4e): a grant scoped to
		// list/corpus, minted through the residency policy. Verifying it per query
		// means Halt revokes retrieval too, and a frontier-bound reader would be
		// refused at issuance (fail-closed: no grant -> no search).
		readerGrant, _ = authz.Issue(controlplane.Capability{Subject: "rag-reader", Action: controlplane.ActionList, Resource: "corpus", Tenant: "public"}, 30*24*time.Hour)
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
	// Governed retrieval (RAG read knobs): top-k per corpus was a hardcoded literal
	// in the chat path; now a governed, versioned, bundle-referenced artifact.
	retrievalDefaults := map[string]controlplane.RetrievalConfig{
		"public": {K: 5},
	}
	// Governed output grammar (output-schema dimension): the extractor's GBNF was a
	// const; now a governed, versioned, bundle-referenced artifact (a swapped grammar
	// silently changes the output contract).
	grammarDefaults := map[string]string{
		"extractor": extractor.EventArrayGBNF,
	}
	// Governed model bindings: the logical model each role routes to. Was two config
	// keys (extractor_model / chat_model); now a governed, versioned, bundle-referenced
	// artifact. A local→frontier swap is where the residency rule bites.
	modelDefaults := map[string]string{
		"extractor": "planner",
		"chat":      "planner",
	}
	// Skills supply + governed approvals (the skills dimension's live consumer): the
	// loader signs its catalog at boot; the plane records which versions the operator
	// approved. Build the loader first for the catalog names, then the plane, then
	// attach (plane needs names, loader needs plane).
	skillSupply, skillNames := newSkillLoader()
	var policies *controlplane.Policies
	var budgets *controlplane.Budgets
	var retrieval *controlplane.Retrieval
	var grammars *controlplane.Grammars
	var models *controlplane.Models
	var skillsPlane *controlplane.Skills
	if db, ierr := datastore.Open(filepath.Join(*drop, "inventory.db")); ierr == nil {
		inv = db
		defer inv.Close()
		prompts = controlplane.GovernedPrompts(promptDefaults, inv, audit)
		policies = controlplane.GovernedPolicies(policyDefaults, inv, audit)
		sampling = controlplane.GovernedSampling(samplingDefaults, inv, audit)
		budgets = controlplane.GovernedBudgets(budgetDefaults, inv, audit)
		retrieval = controlplane.GovernedRetrieval(retrievalDefaults, inv, audit)
		grammars = controlplane.GovernedGrammars(grammarDefaults, inv, audit)
		models = controlplane.GovernedModels(modelDefaults, inv, audit)
		skillsPlane = controlplane.GovernedSkills(skillNames, inv, audit)
	} else {
		prompts = controlplane.NewPrompts(promptDefaults)
		policies = controlplane.NewPolicies(policyDefaults)
		sampling = controlplane.NewSampling(samplingDefaults)
		budgets = controlplane.NewBudgets(budgetDefaults)
		retrieval = controlplane.NewRetrieval(retrievalDefaults)
		grammars = controlplane.NewGrammars(grammarDefaults)
		models = controlplane.NewModels(modelDefaults)
		skillsPlane = controlplane.NewSkills(skillNames)
	}
	skillSupply.attach(skillsPlane)
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
	// Activating the "extractor" grammar drives the live GBNF the extractor sends.
	baseGramOnActivate := grammars.OnActivate
	grammars.OnActivate = func(gv controlplane.GrammarVersion) {
		if baseGramOnActivate != nil {
			baseGramOnActivate(gv)
		}
		if gv.Name == "extractor" {
			extractor.SetGrammar(gv.Text)
		}
	}
	// Activating the "extractor" model binding drives the live extraction model.
	baseModOnActivate := models.OnActivate
	models.OnActivate = func(mv controlplane.ModelVersion) {
		if baseModOnActivate != nil {
			baseModOnActivate(mv)
		}
		if mv.Name == "extractor" {
			extractor.SetModel(mv.Model)
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
	var modelCatalogSvc server.ModelCatalogStore
	var eventsSvc server.EventStore
	var summariesSvc server.SummaryStore
	var itemsSvc server.ItemStore
	var ragLabSvc server.RAGLab
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
		if cfgJSON, ok, _ := inv.LatestRetrieval("public"); ok {
			var cfg controlplane.RetrievalConfig
			if json.Unmarshal([]byte(cfgJSON), &cfg) == nil {
				retrieval.Rehydrate("public", cfg)
			}
		}
		if text, ok, _ := inv.LatestGrammarText("extractor"); ok {
			grammars.Rehydrate("extractor", text)
		}
		for _, n := range []string{"extractor", "chat"} {
			if m, ok, _ := inv.LatestModel(n); ok && m != "" {
				models.Rehydrate(n, m)
			}
		}
		for _, n := range skillNames {
			if h, ok, _ := inv.LatestSkillPin(n); ok && h != "" {
				skillsPlane.Rehydrate(n, h)
			}
		}
		// Model catalog: the DB is the single source of truth for which models exist
		// (URL/SHA/file/port/ctx). Seed the fail-closed bootstrap on first boot; never
		// clobber operator edits. modeld/prepareassets read this same catalog via -db.
		if _, err := inv.SeedModelCatalogIfEmpty(modelcatalog.DefaultSeed()); err != nil {
			log.Printf("webapp: seed model catalog: %v", err)
		}
		modelCatalogSvc = inv                      // *datastore.Store satisfies server.ModelCatalogLister
		eventsSvc = eventProjection{inv: inv}      // DB-backed calendar projection
		summariesSvc = summaryProjection{inv: inv} // DB-backed tasks/digest/directory projection
		itemsSvc = itemProjection{inv: inv}        // DB-backed DEDUPED item projection
		// Kill switch must survive a restart: rehydrate the persisted level on boot and
		// persist every change. An engaged halt that resets on restart is not a halt.
		if v, ok, _ := inv.GetConfig("kill_level"); ok {
			if n, err := strconv.Atoi(v); err == nil {
				safety.Rehydrate(controlplane.KillLevel(n))
			}
		}
		safety.OnSet = func(l controlplane.KillLevel) { _ = inv.SetConfig("kill_level", strconv.Itoa(int(l))) }
		// Bring the local models up automatically on startup (planner/coder + gateway),
		// so chat & extraction work out of the box — then seed the example emails so they
		// are extracted via the LLM (clean) rather than the regex fallback (noisy). The
		// model load runs in the background so the UI comes up immediately; emails seed
		// once the gateway is ready. -automodels=false (or WEBAPP_AUTOMODELS=false) skips
		// the auto-start and seeds with offline extraction.
		modelCatalog, _ := inv.ListModelCatalog()
		// RAG lab: tune ingestion (chunker/size/overlap/mode/embedder) + reindex. Needs
		// the writable corpus; embedder options are the catalog models (+ "none").
		if corpusWritable != nil {
			embedders := []string{"none"}
			for _, m := range modelCatalog {
				embedders = append(embedders, m.Name)
			}
			embedMgr := newEmbedManager(ctx, modelstack.BinPath(), *assetsDir, modelCatalog)
			addCleanup(embedMgr.stop)
			ragLabSvc = newRAGLab(inv, corpusWritable, cfg.Processed, embedders, ragApplyEmbed(corpusWritable, reader, embedMgr, &ragSemantic))
		}
		startModelsAndSeed := func() {
			if gatewayReachable(dial) {
				os.Setenv("EXTRACT_MODE", "llm")
				log.Printf("webapp: gateway already up on %s — extracting via the LLM", dial)
				seedEmails(inv, cfg.Inbox, cfg.Outbox, *seedDir)
				return
			}
			if ok, reason := modelstack.Available(*assetsDir, modelCatalog); !ok {
				log.Printf("webapp: models not auto-started (%s) — using offline extraction", reason)
				seedEmails(inv, cfg.Inbox, cfg.Outbox, *seedDir)
				return
			}
			log.Printf("webapp: bringing up local models in the background (first load is slow)…")
			stack, err := modelstack.Start(ctx, *assetsDir, dial, modelCatalog, 180*time.Second)
			if err == nil {
				addCleanup(stack.Close)
			}
			if err != nil {
				log.Printf("webapp: model auto-start failed (%v) — using offline extraction", err)
				seedEmails(inv, cfg.Inbox, cfg.Outbox, *seedDir)
				return
			}
			os.Setenv("EXTRACT_MODE", "llm") // LLM-only extraction (no noisy regex fallback)
			log.Printf("webapp: models up on %s — seeding + extracting emails via the LLM", dial)
			seedEmails(inv, cfg.Inbox, cfg.Outbox, *seedDir)
		}
		if *autoModels {
			go startModelsAndSeed()
		} else {
			seedEmails(inv, cfg.Inbox, cfg.Outbox, *seedDir)
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
		extractor.SetGrammar(grammars.Text("extractor"))
		extractor.SetModel(models.Bound("extractor")) // governed model binding (was extractor_model config key)
		sc := sampling.Config("extractor")
		extractor.SetSampling(sc.Temperature, sc.MaxTokens, sc.Seed)
		// grammar JSON mode (C5) from governed config (DB is the source of truth).
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
		bundleSvc = bundleStore{inv: inv, audit: audit, prompts: prompts, sampling: sampling, policies: policies, budgets: budgets, retrieval: retrieval, grammars: grammars, models: models}
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

	// Chat is constructed late (after the governed planes exist) but only when the
	// corpus opened — reader nil = no Chat tab. Same reader NHI grant as search.
	var chatSvc server.ChatService
	var chatHistorySvc server.ChatHistoryStore
	if reader != nil {
		// The chat model's context window (for the usage bar): resolve the governed
		// "chat" binding to its catalog ctx size, fail-closed to 8192.
		chatCtxLimit := func() int {
			name := "planner"
			if models != nil {
				if b := models.Bound("chat"); b != "" {
					name = b
				}
			}
			if inv != nil {
				if e, ok, _ := inv.GetModelCatalog(name); ok && e.Ctx > 0 {
					return e.Ctx
				}
			}
			return 8192
		}
		chatSvc = &chatService{reader: reader, grant: readerGrant, authz: authz, gw: gw, prompts: prompts, sampling: sampling, models: models, retrieval: retrieval, semantic: &ragSemantic, hist: inv, ctxLimit: chatCtxLimit}
		if inv != nil {
			chatHistorySvc = chatHistoryStore{inv: inv}
		}
	}

	srv := &http.Server{
		Addr: *addr,
		Handler: server.New(server.Config{
			AuditPath: auditPath, OutboxDir: cfg.Outbox, InboxPath: cfg.Inbox,
			Egress: egress, Fetch: fetch, Verifier: verifier, Safety: safety, Search: search, Index: enrichIndex,
			Flywheel: flywheelSvc, Chat: chatSvc, ChatHistory: chatHistorySvc,
			Authz: authz, OperatorToken: opToken, AppToken: appToken, Audit: audit,
			MCP: mcpReg, Prompts: prompts, Policies: policies, Sampling: sampling, Budgets: budgets, Retrieval: retrieval, Grammars: grammars, Models: models,
			Skills: skillsPlane, SkillCatalog: skillSupply, ModelCatalog: modelCatalogSvc, Events: eventsSvc, Summaries: summariesSvc, Items: itemsSvc, RAG: ragLabSvc,
			AssetsDir: *assetsDir, GatewayURL: *gwFlag, GatewayUp: func() bool { return gw != nil && gw.Up() }, IdentityEphemeral: !persistentID,
			Eval:    evalSvc,
			Bundles: bundleSvc,
			Profile: profileSvc,
		}).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      120 * time.Second, // live eval/chat can be slow
		IdleTimeout:       120 * time.Second,
	}
	fmt.Printf("agent console: http://%s\n", *addr)
	fmt.Printf("drop .txt emails into: %s\n", cfg.Inbox)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("webapp: %v", err)
		}
	}()

	// Block until a signal cancels ctx, then shut down DETERMINISTICALLY: stop the
	// HTTP server, then run the model cleanups synchronously (each waits for its
	// spawned llama.cpp process group to actually die) BEFORE returning. No orphans.
	<-ctx.Done()
	log.Println("webapp: shutting down — stopping model servers…")
	shutCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	_ = srv.Shutdown(shutCtx)
	cancel()
	cleanupMu.Lock()
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
	cleanupMu.Unlock()
	log.Println("webapp: stopped.")
}

// agentIdentity loads (or creates) the persistent ed25519 seed at path and returns
// the signer, a verifier trusting that key, and whether the identity is PERSISTENT
// (the seed is backed on disk and will survive a restart). persistent is false when
// the seed could not be read AND a freshly generated one could not be written, or the
// seed on disk is unusable — in both cases the key lives only in memory, so every .ics
// signed this run stops verifying after a restart. It never degrades silently: the
// caller decides (fail closed by default; opt-in for a throwaway dev run). err is
// returned only when no signer can be constructed at all.
func agentIdentity(path string) (signer *provenance.Signer, verifier *provenance.Verifier, persistent bool, err error) {
	const keyID = "email-agent"
	persistent = true
	seed, rerr := os.ReadFile(path)
	if rerr != nil || len(seed) != ed25519.SeedSize {
		seed = make([]byte, ed25519.SeedSize)
		if _, gerr := rand.Read(seed); gerr != nil {
			return nil, nil, false, fmt.Errorf("generate agent seed: %w", gerr)
		}
		// First boot (or a replaced seed): persist it so the next boot reuses it. A
		// write failure means the key is in-memory only — NOT persistent.
		if werr := os.WriteFile(path, seed, 0o600); werr != nil {
			persistent = false
		}
	}
	signer, pub, serr := provenance.SignerFromSeed(keyID, seed)
	if serr != nil {
		// The seed is unusable; fall back to an ephemeral key so a signer exists, but
		// flag it as non-persistent for the caller to surface/refuse.
		signer, pub, serr = provenance.NewSigner(keyID)
		if serr != nil {
			return nil, nil, false, fmt.Errorf("agent signer: %w", serr)
		}
		persistent = false
	}
	return signer, provenance.NewVerifier().Trust(keyID, pub), persistent, nil
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

// loadDotenv seeds the process environment from a KEY=VALUE file (dotenv). A real
// environment variable always wins over the file (12-factor), and a missing file is
// fine. No dependency — a few lines of parsing, called once before flags. Values may
// be quoted; lines starting with # are comments.
func loadDotenv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, set := os.LookupEnv(k); !set { // real env wins over the file
			_ = os.Setenv(k, v)
		}
	}
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
func chatParams(sampling *controlplane.Sampling, models *controlplane.Models) gateway.ChatParams {
	p := gateway.ChatParams{Model: "planner", Temperature: 0.2, MaxTokens: 400}
	if sampling != nil {
		if sc := sampling.Config("chat"); sc.MaxTokens > 0 {
			p.Temperature, p.MaxTokens = sc.Temperature, sc.MaxTokens
		}
	}
	if models != nil {
		if m := models.Bound("chat"); m != "" {
			p.Model = m // governed model binding (was the chat_model config key)
		}
	}
	return p
}
