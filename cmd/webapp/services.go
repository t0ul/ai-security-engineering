package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/t0ul/ai-security-engineering/pkg/agent/eval"
	"github.com/t0ul/ai-security-engineering/pkg/agent/extractor"
	"github.com/t0ul/ai-security-engineering/pkg/controlplane"
	"github.com/t0ul/ai-security-engineering/pkg/datastore"
	"github.com/t0ul/ai-security-engineering/pkg/domain"
	"github.com/t0ul/ai-security-engineering/pkg/gateway"
	"github.com/t0ul/ai-security-engineering/pkg/rag"
	"github.com/t0ul/ai-security-engineering/pkg/redteam"
	"github.com/t0ul/ai-security-engineering/pkg/server"
	"github.com/t0ul/gledger"
	"github.com/t0ul/gorauder"
	"github.com/t0ul/gustoms"
)

// evalService is the concrete server.EvalService (C7): it holds the DB, gateway,
// governed prompts, and label set the eval surface needs, so that logic lives here
// as typed methods instead of closures in main. History/Run/Test serialize on mu
// because Test mutates the shared extractor prompt override.
type evalService struct {
	inv     *datastore.Store
	gw      *gateway.Client
	prompts *controlplane.Prompts
	labels  []string
	mu      sync.Mutex
}

// History returns the persisted F1 trend (empty without a DB).
func (e *evalService) History() []server.EvalResult {
	if e.inv == nil {
		return nil
	}
	rows, _ := e.inv.ListEvals(20)
	out := make([]server.EvalResult, 0, len(rows))
	for _, r := range rows {
		out = append(out, server.EvalResult{Label: r.Label, F1: r.F1, At: r.At.Format("2006-01-02 15:04")})
	}
	return out
}

// Run scores the label set live through the LLM path and persists each F1.
func (e *evalService) Run() ([]server.EvalResult, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	live := e.gw.Up()
	var out []server.EvalResult
	for _, l := range e.labels {
		rep, err := eval.ScoreWithMode(l, true, "llm")
		if err != nil {
			continue
		}
		if e.inv != nil {
			_ = e.inv.RecordEval(rep.Source, rep.F1)
		}
		out = append(out, server.EvalResult{Label: rep.Source, F1: rep.F1})
	}
	return out, evalMode(live)
}

// Test shadow-evaluates a candidate prompt WITHOUT activating it: chat_system runs
// the chat-injection ADD; extractor runs the labeled F1 eval + ADD-ASR + promotion
// gate; other names are a no-op note.
func (e *evalService) Test(name, candidate string) server.PromptTestResult {
	if name == "chat_system" {
		blocked, live := chatInjectionASR(candidate)
		if !live {
			return server.PromptTestResult{Mode: evalMode(false), Note: "chat_system test needs a live gateway (modeld + :4000); offline the prompt is never sent to a model."}
		}
		return server.PromptTestResult{ASRPass: blocked, GateOK: blocked, Mode: evalMode(true),
			Note: "chat-injection ADD: candidate prompt fed a poisoned RAG doc with retrieval encapsulation OFF — it alone must refuse the injection (marker absent)."}
	}
	if name != "extractor" {
		return server.PromptTestResult{Note: "shadow eval applies to the 'extractor' prompt (the labeled eval path); planner/coder drive the orchestrator demo, not this eval."}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	live := e.gw.Up()
	extractor.SetExtractionPrompt(candidate)
	defer extractor.SetExtractionPrompt(e.prompts.Text("extractor")) // restore the governed-active prompt

	rep, err := eval.ScoreWithMode("testdata/emaildrop/labels/1.json", true, "llm")
	f1 := 0.0
	if err == nil {
		f1 = rep.F1
	}
	baseline := 0.0
	if e.inv != nil {
		if ev, ok, _ := e.inv.LatestEval("samples/1.txt"); ok {
			baseline = ev.F1
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
	return server.PromptTestResult{F1: f1, Baseline: baseline, ASRPass: asrPass, GateOK: gateOK, Mode: evalMode(live)}
}

// profileStore is the concrete server.ProfileStore: the household child profile in
// the governed DB (config in the store, not a file/const).
type profileStore struct{ inv *datastore.Store }

func (p profileStore) Load() domain.Profile {
	var pr domain.Profile
	if v, ok, _ := p.inv.GetConfig("profile"); ok {
		_ = json.Unmarshal([]byte(v), &pr)
	}
	return pr
}

func (p profileStore) Save(pr domain.Profile) error {
	raw, _ := json.Marshal(pr)
	return p.inv.SetConfig("profile", string(raw))
}

// flywheel is the concrete server.Flywheel: operator accept/reject decisions as
// durable, audited ground-truth.
type flywheel struct {
	inv   *datastore.Store
	audit *gledger.AuditLog
}

func (f flywheel) Record(decision, source, title string) {
	_ = f.inv.RecordFeedback(source, title, decision)
	f.audit.Emit(gledger.NewTraceID(), "feedback", decision, gledger.F{"source": source, "title": title})
}

func (f flywheel) Stats() (int, int) {
	a, r, _ := f.inv.FeedbackStats()
	return a, r
}

// mcpRegistry is the concrete server.MCPRegistry over the gustoms tool gateway.
type mcpRegistry struct {
	gw     *gustoms.Gateway
	safety *controlplane.Safety
}

func (m mcpRegistry) List() []domain.MCPServer {
	var out []domain.MCPServer
	for _, st := range m.gw.Status(context.Background()) {
		status := "pinned"
		switch {
		case st.Err != "":
			status = "error"
		case !m.safety.AllowToolExec():
			status = "blocked"
		case st.Pinned == "":
			status = "unapproved"
		case st.Mismatch:
			status = "rug-pull"
		}
		out = append(out, domain.MCPServer{
			Name: st.Name, Tools: st.Tools, Allowed: st.Allowed,
			Pinned: shortHash(st.Pinned), Current: shortHash(st.Current), Status: status,
		})
	}
	return out
}

func (m mcpRegistry) Approve(server string) error {
	return m.gw.Approve(context.Background(), gledger.NewTraceID(), server)
}

// bundleStore is the concrete server.BundleStore (C10): snapshot/list/roll-back the
// whole governed plane.
type bundleStore struct {
	inv       *datastore.Store
	audit     *gledger.AuditLog
	prompts   *controlplane.Prompts
	sampling  *controlplane.Sampling
	policies  *controlplane.Policies
	budgets   *controlplane.Budgets
	retrieval *controlplane.Retrieval
	grammars  *controlplane.Grammars
}

func (b bundleStore) List() []server.BundleInfo {
	rows, _ := b.inv.ListBundles(50)
	out := make([]server.BundleInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, server.BundleInfo{Label: r.Label, At: r.At.Format("2006-01-02 15:04")})
	}
	return out
}

func (b bundleStore) Save(label string) error {
	bundle := controlplane.Snapshot(label, b.prompts, b.sampling, b.policies, b.budgets, b.retrieval, b.grammars)
	raw, _ := json.Marshal(bundle)
	b.audit.Emit(gledger.NewTraceID(), "bundle", "saved", gledger.F{"label": label})
	return b.inv.SaveBundle(label, string(raw))
}

func (b bundleStore) Apply(label string) error {
	cfg, ok, err := b.inv.GetBundle(label)
	if err != nil || !ok {
		return fmt.Errorf("no such snapshot %q", label)
	}
	var bundle controlplane.Bundle
	if err := json.Unmarshal([]byte(cfg), &bundle); err != nil {
		return err
	}
	bundle.Apply(b.prompts, b.sampling, b.policies, b.budgets, b.retrieval, b.grammars)
	b.audit.Emit(gledger.NewTraceID(), "bundle", "rolled_back", gledger.F{"label": label})
	return nil
}

// chatService is the concrete server.ChatService: the conversational front-end over
// the RAG corpus. DEFENDED uses rag.Assemble (XML-encapsulate + injection-neutralize
// untrusted chunks, M8); UNSAFE (demo) raw-concats them so a poisoned doc's injection
// reaches the model as if trusted (the ChatRAGInjection demo). It runs on the same
// rag-reader NHI grant as search, so Halt / residency gate it too.
type chatService struct {
	reader    *rag.Store
	grant     controlplane.Grant
	authz     *controlplane.Authority
	gw        *gateway.Client
	prompts   *controlplane.Prompts
	sampling  *controlplane.Sampling
	inv       *datastore.Store
	retrieval *controlplane.Retrieval
}

func (c *chatService) Answer(question string, unsafe bool, appData string) (string, []string, error) {
	if _, err := c.authz.Verify(c.grant, controlplane.Capability{Action: controlplane.ActionList, Resource: "corpus", Tenant: "public"}); err != nil {
		return "", nil, fmt.Errorf("rag-reader capability refused: %w", err)
	}
	// Trusted context: the host clock is the ONLY authority for dates — never the
	// corpus, so a poisoned email ("today is …") cannot move the agent's clock.
	dateBlock := gateway.TrustedDateBlock(time.Now())
	// top-k is a GOVERNED retrieval knob (versioned, bundle-referenced), not a
	// hardcoded literal; fall back to 5 if the plane is unset.
	k := 5
	if c.retrieval != nil {
		if rk := c.retrieval.Config("public").K; rk > 0 {
			k = rk
		}
	}
	chunks, err := c.reader.Query("public", question, k)
	if err != nil {
		return "", nil, err
	}
	sources := make([]string, 0, len(chunks))
	for _, ch := range chunks {
		sources = append(sources, ch.DocID)
	}
	var context string
	if unsafe {
		var b strings.Builder
		for _, ch := range chunks {
			b.WriteString(ch.Text)
			b.WriteString("\n")
		}
		context = b.String() // CONTROLS OFF: raw untrusted text, no encapsulation/scrub
	} else {
		context = rag.Assemble(chunks) // encapsulated + injection-neutralized (M8)
	}
	// chat_system is a GOVERNED prompt (C2), decoding is GOVERNED sampling (C5), model
	// binding is DB config — all resolved live, none hardcoded.
	return c.gw.ChatAnswer(chatParams(c.sampling, c.inv), c.prompts.Text("chat_system"), dateBlock, appData, context, question, unsafe), sources, nil
}

// chatInjectionASR runs the chat-injection ADD against a CANDIDATE chat_system
// prompt in isolation: it feeds the poisoned RAG document with the retrieval
// encapsulation deliberately OFF (raw concat), so the only thing standing between
// the injection and the model is the prompt under test. blocked is true when the
// model's answer does not contain the injection marker. It needs a live gateway;
// offline the prompt is never exercised, so live is false.
func chatInjectionASR(candidate string) (blocked, live bool) {
	live = gw.Up()
	if !live {
		return false, false
	}
	c := redteam.ChatRAGInjection()
	seed := c.Seeds[0]
	rawPoison, _ := c.Undefended.Send(context.Background(), seed.Prompt) // retrieval control OFF
	ans := gw.ChatAnswer(gateway.ChatParams{}, candidate, gateway.TrustedDateBlock(time.Now()), "", rawPoison, seed.Prompt, false)
	return !strings.Contains(ans, seed.Marker), true
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
