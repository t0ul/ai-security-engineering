package controlplane

import (
	"context"
	"regexp"
	"strings"

	"github.com/t0ul/gledger"
	"github.com/t0ul/gonductor"
)

// MaxSessionSteps is the app-level circuit breaker: the planner halts the loop
// once this many steps have run, before the engine's hard cap. It guards against
// a runaway / sponge loop (OWASP LLM DoS).
const MaxSessionSteps = 5

// haltSentinel marks a plan that upstream controls have aborted. Downstream
// nodes short-circuit on it, exactly as the Python loop did.
const haltSentinel = "EXECUTION_HALTED"

const haltMessage = "Execution halted by security controls."

var (
	commandRe     = regexp.MustCompile(`COMMAND:\s*(.+)`)
	commandLineRe = regexp.MustCompile(`(?im)^\s*COMMAND:.*$`)
	imageRe       = regexp.MustCompile(`!\[.*?\]\(.*?\)`)
)

// State is the CaMeL loop state carried node to node.
type State struct {
	TraceID       string
	RawHistory    string
	BlogPlan      string
	ToolCommand   string
	ToolOutput    string
	FinalMarkdown string
	StepCount     int
}

// Approver is the human-in-the-loop gate. It returns true to proceed to
// execution. A nil Approver denies by default (HITL is not silently bypassed).
type Approver func(state State) bool

// Orchestrator wires the CaMeL graph: the privileged planner chooses control
// flow (outline + one read-only command), a HITL gate approves it, the executor
// detonates it in the sandbox, and the quarantined coder formats the untrusted
// output. The planner never sees raw sandbox output; the coder never executes.
type Orchestrator struct {
	LLM     *LLMClient
	Interp  *Interpreter
	Audit   *gledger.AuditLog
	Approve Approver
	// GatewayModels names the logical models the gateway allowlists; defaults to
	// {"planner","coder"}.
	PlannerModel string
	CoderModel   string
	// Killed, when set and returning true, halts the loop before any work — the
	// governed kill switch (wire it to Governance.Killed). Fail-closed.
	Killed func() bool
	// Safety, when set, is the layered kill switch: it gates whether a request
	// may start (AllowRequest) and whether tools may execute (AllowToolExec).
	Safety SafetyGate
}

// SafetyGate is the layered kill switch the loop consults; *Safety implements it.
type SafetyGate interface {
	AllowRequest() bool
	AllowToolExec() bool
}

func (o *Orchestrator) plannerModel() string {
	if o.PlannerModel != "" {
		return o.PlannerModel
	}
	return "planner"
}

func (o *Orchestrator) coderModel() string {
	if o.CoderModel != "" {
		return o.CoderModel
	}
	return "coder"
}

// Run builds and invokes the graph for one request under traceID.
func (o *Orchestrator) Run(ctx context.Context, traceID, rawHistory string) (State, error) {
	if o.Killed != nil && o.Killed() {
		o.Audit.Emit(traceID, "request", "killswitch_halt", gledger.F{})
		return State{TraceID: traceID, RawHistory: rawHistory, BlogPlan: haltSentinel, FinalMarkdown: haltMessage}, nil
	}
	if o.Safety != nil && !o.Safety.AllowRequest() {
		o.Audit.Emit(traceID, "request", "safety_halt", gledger.F{})
		return State{TraceID: traceID, RawHistory: rawHistory, BlogPlan: haltSentinel, FinalMarkdown: haltMessage}, nil
	}
	g := gonductor.New[State]().
		AddNode("planner", o.planner).
		AddNode("approval", o.approval).
		AddNode("executor", o.executor).
		AddNode("coder", o.coder).
		SetEntry("planner").
		AddEdge("planner", "approval").
		AddEdge("approval", "executor").
		AddEdge("executor", "coder").
		AddEdge("coder", gonductor.END).
		MaxSteps(16). // hard backstop; the app-level breaker trips first
		OnStep(func(gc *gonductor.Context, node string, _ State) {
			o.Audit.Emit(gc.TraceID, "graph", "enter", gledger.F{"node": node, "step": gc.Step})
		})
	c, err := g.Compile()
	if err != nil {
		return State{}, err
	}
	gctx := &gonductor.Context{Context: ctx, TraceID: traceID}
	return c.Invoke(gctx, State{TraceID: traceID, RawHistory: rawHistory})
}

func (o *Orchestrator) planner(ctx *gonductor.Context, s State) (State, error) {
	// App-level circuit breaker: halt before doing more work.
	if s.StepCount >= MaxSessionSteps {
		o.Audit.Emit(s.TraceID, "planner", "circuit_breaker_tripped", gledger.F{"step_count": s.StepCount})
		s.BlogPlan = haltSentinel
		return s, nil
	}
	plan, err := o.LLM.Complete(ctx, s.TraceID, o.plannerModel(), PlannerSystemPrompt, s.RawHistory,
		CompleteOpts{Temperature: 0.1, MaxTokens: 250, Stop: []string{"<|eot_id|>", "<|end_of_text|>"}})
	if err != nil {
		return s, err
	}
	s.BlogPlan = plan
	s.StepCount++
	o.Audit.Emit(s.TraceID, "planner", "plan_captured", gledger.F{"step_count": s.StepCount})
	return s, nil
}

func (o *Orchestrator) approval(_ *gonductor.Context, s State) (State, error) {
	if s.BlogPlan == haltSentinel {
		return s, nil
	}
	approved := o.Approve != nil && o.Approve(s)
	o.Audit.Emit(s.TraceID, "approval", "decision", gledger.F{"approved": approved})
	if !approved {
		s.BlogPlan = haltSentinel
	}
	return s, nil
}

func (o *Orchestrator) executor(ctx *gonductor.Context, s State) (State, error) {
	if s.BlogPlan == haltSentinel {
		s.ToolOutput = ""
		s.FinalMarkdown = haltMessage
		return s, nil
	}
	// CaMeL: the privileged planner chose the command (control flow). Fall back to
	// a harmless read-only probe if none was emitted.
	command := "uname -a"
	source := "fallback"
	if m := commandRe.FindStringSubmatch(s.BlogPlan); m != nil {
		command = strings.TrimSpace(m[1])
		source = "planner"
	}
	o.Audit.Emit(s.TraceID, "executor", "command_selected", gledger.F{"command": command, "source": source})
	s.ToolCommand = command
	// Layered kill switch: at LevelBlockTools and above, planning stands but no
	// side effect runs — the command is never detonated.
	if o.Safety != nil && !o.Safety.AllowToolExec() {
		o.Audit.Emit(s.TraceID, "executor", "blocked_by_safety", gledger.F{"command": command})
		s.ToolOutput = "[execution blocked by safety level]"
		s.StepCount++
		return s, nil
	}
	s.ToolOutput = o.Interp.ExecuteInSandbox(ctx, s.TraceID, command)
	s.StepCount++
	return s, nil
}

func (o *Orchestrator) coder(ctx *gonductor.Context, s State) (State, error) {
	if s.BlogPlan == haltSentinel {
		s.FinalMarkdown = haltMessage
		return s, nil
	}
	// Keep the control command out of the prose; the coder sees only the UNTRUSTED
	// sandbox output and raw logs (CaMeL: untrusted data reaches only the Q-LLM).
	planForBlog := strings.TrimSpace(commandLineRe.ReplaceAllString(s.BlogPlan, ""))
	user := "Write a Markdown blog post using this structure:\n" + planForBlog +
		"\n\nVerified command output captured from the sandbox:\n" + s.ToolOutput +
		"\n\nRaw session logs:\n" + s.RawHistory
	raw, err := o.LLM.Complete(ctx, s.TraceID, o.coderModel(), CoderSystemPrompt, user,
		CompleteOpts{Temperature: 0.1, MaxTokens: 600, Stop: []string{"<|im_end|>", "<|endoftext|>"}})
	if err != nil {
		return s, err
	}
	safe, stripped := SanitizeMarkdown(raw)
	o.Audit.Emit(s.TraceID, "coder", "sanitize", gledger.F{"images_stripped": stripped, "out_len": len(safe)})
	s.FinalMarkdown = safe
	s.StepCount++
	return s, nil
}

// SanitizeMarkdown strips markdown image tags to block URL-pixel exfiltration
// (the ![alt](url) data-exfil PoC) and returns the count removed. Exported so
// the red-team harness can measure its ASR against the exfil technique.
//
// NOTE: a single-pass regex, NOT a real HTML/DOM sanitizer. Reference-style
// images, raw <img>, and autolinks are NOT covered — treat as one layer only.
func SanitizeMarkdown(md string) (string, int) {
	stripped := len(imageRe.FindAllString(md, -1))
	safe := imageRe.ReplaceAllString(md, "[IMAGE BLOCKED BY SANITIZER]")
	return safe, stripped
}
