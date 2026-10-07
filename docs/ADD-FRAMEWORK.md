# `add` — Attack-Driven Development for Go AI systems

A Go library that makes proving an attack as natural as writing a unit test. It
rides `go test`; it is not a scanner, a CLI, or a product.

> **The rule: no control without a proven attack.** Every control ships as a pair
> — the attack landing *without* it (undefended ASR 100% → the threat is real) and
> the same attack *failing with* it (defended 0%). That before→after is the *why*
> behind every security feature. The full control↔attack↔evidence ledger is
> `docs/WHY-CONTROLS.md`; the living list is `go run ./cmd/scorecard`.

> **Two layers — offline (default) and live (`-tags live`).**
>
> **Offline** (default `go test ./...`, `cmd/scorecard`): each case is a white-box,
> in-process pair of `gorauder.TargetFunc` closures — an *undefended* target that
> lets the attack land (ASR 100%) and a *defended* target running the real control
> (ASR 0%). Deterministic, fast, CI-able, no model. "Scorecard green" means the
> controls hold against the attack patterns — **not** that attacks ran against a
> live model.
>
> **Live** (`go test -tags live ./livetest/ -v`): the model-dependent checks run
> against the REAL llama/qwen stack (`internal/livemodel` boots the models + the
> in-process gouncer gateway, same as `cmd/livecheck`). Today: `TestLiveEvalF1`
> (live extraction scored against the labeled emails — 3.txt F1=1.00, 1.txt
> F1=1.00) and `TestLiveExtractionInjectionDefended` (a real indirect-injection
> email through the live extractor). Honest rule for live ADD: **gate on the
> defended path** (guard-sanitized input must not yield the attacker's event) and
> **observe/log the undefended ASR** — a live 3B model may resist a given payload
> on its own, so asserting undefended==100% would be a fake invariant. Live tests
> Skip (not fail) when the models aren't set up.
>
> Run `go run ./cmd/preflight` to see offline-vs-live and whether models are
> downloaded/serving. Not every test should be live: deterministic units (parsers,
> control plane, crypto) need no model; only model-dependent behavior (extraction,
> injection, hallucination, eval F1) gets a live variant in `livetest/`.

## Name
Package `add` (Attack-Driven Development). Reads naturally in tests —
`add.Run(t, tech)`, `add.Gate(t)` — the way `require`/`assert`/`is` do. Fleet
fallback: `goadd`. Not `GOAISEC` (implies a whole platform) or `SDD` (vaguer,
drops the attack/proof framing).

## What it IS
- A **library + `go test` helpers** for expressing a security control as a pair:
  an attack that proves the weakness and the control that closes it.
- **Proof by outcome**: a technique passes only when an *oracle* observes the
  real impact (marker reached, canary leaked, egress attempted, side effect
  seen) — not when a response merely "looks safe".
- A **regression gate**: a control that silently stops being wired flips the
  defended ASR above zero and reds the build.
- A **coverage map**: which OWASP LLM/Agentic risks have a proven-and-defended
  technique, and which don't (it surfaces *absence*).

## What it is NOT
- **Not a black-box scanner / external pentest tool.** White-box, in-process,
  operated by the embedded team on their own code.
- **Not SAST/SCA/secrets-scanning.** Those are static finding-generators; bolt
  existing tools (semgrep/trivy/gitleaks) into CI separately. `add` is dynamic
  and proves exploitation.
- **Not a vuln-report generator.** Output is a reproducible PoC + an ASR number,
  not a severity list.
- **Not a DSL.** Techniques are Go funcs. If it ever wants a YAML config, that's
  a design failure (friction returns).
- **Not a jailbreak-prompt zoo.** Prompts/seeds live in gorauder; `add` is about
  the attack→oracle→defense contract around them.

## Setup expectations (the preconditions)
You are building a Go AI/agentic product and want security in the dev loop. To
use `add`, each technique must provide three things — this is the cost of
honesty, and it's deliberate:
1. **A defended target** — the real control path in your code (a function, or
   the whole pipeline).
2. **An undefended counterfactual** — the same path with the control removed.
   You must keep this reachable in tests. It's what proves your attack is real
   rather than a tautology.
3. **An oracle** — a predicate that observes impact. Compose from the provided
   helpers; write a bespoke one when needed.
Techniques live beside the code as `*_add_test.go` and run under `go test`.
No separate runner, no CI plumbing, no config file.

## Core API (sketch)
```go
type Signal  = any                 // whatever the attack produced (output, trace, side effect)
type Outcome struct{ Breached bool; Evidence string }

type Target interface {            // the thing under attack
    Exercise(ctx context.Context, seed gorauder.Seed) (Signal, error)
}

type Technique struct {
    Name       string
    Risk       string               // OWASP tag, e.g. "LLM01", "ASI07"
    Seeds      []gorauder.Seed
    Undefended Target                // control removed
    Defended   Target                // real control
    Oracle     func(Signal) Outcome  // did impact happen?
}

func Run(t *testing.T, tech Technique)   // asserts the invariant, records ASR
func Gate(t *testing.T, s ...Technique)  // run many; write coverage + ASR JSON
```

## The invariant (the whole point)
`Run` asserts BOTH sides, every time:
- **undefended ASR == 1.0** — the attack genuinely breaches when the control is
  gone (else the test proves nothing);
- **defended ASR == 0.0** — the control blocks it.
A normal unit test checks only the second. The first is what keeps the suite
from rotting into green tautologies. This pairing is why `add` is a library, not
a convention.

## Two target granularities (close the white-box blind spot)
White-box attacks on a *control function* prove the logic, not that it's wired
into the running system. So a technique may run against either, ideally both:
- **unit** — call the control directly (fast, precise; proves the logic);
- **integration** — drive the real pipeline end to end (proves the control is
  actually on the path).
Same technique, two `Target`s. A control that passes unit but fails integration
means "defense exists but isn't wired" — the failure white-box-only testing
misses.

## Oracle vocabulary (helpers, not magic)
`MarkerReached(marker)`, `Blocked(sentinel)`, `CanaryLeaked(token)`,
`EgressAttempted(sensor)`, `SideEffectObserved(probe)`. Composable; a technique
writes its own when these don't fit. Oracles are deliberately NOT generic —
impact is domain-specific.

## Coverage grid
`Gate` emits machine-readable ASR + a risk→technique map. The artifact for an
embedded team is the *gap list*: risks with no proven-and-defended technique.
Track it over time; a regression or a new uncovered risk is visible in CI.

## Migration (mostly refactor, not greenfield)
- `gorauder` stays the seed/converter/scorer engine underneath.
- `redteam.Cases()` → one `add.Technique` each (they already carry
  undefended/defended + a marker; this just formalizes the shape).
- `cmd/scorecard` → `add.Gate` + a thin reporter.
- The existing "ASR 100%→0%" assertions become the built-in invariant.
Net result: adding an attack = writing one `_test.go` technique; running the
whole suite = `go test ./...`.

## Fuzzing — discovery, the complement to regression
ADD proves KNOWN attacks stay blocked. `go fuzz` (coverage-guided) searches for
UNKNOWN inputs that break a control. Both live in `add`, over the same controls:
- Fits controls whose success is a **syntactic invariant on the output**, not a
  planted marker (the marker gets mutated away). So: the sanitizers
  (`ics.SanitizeField`/`SanitizeMarkdown`, `guard`, `argcheck`), the SSRF filter
  (`netpolicy`), traversal guards (`safeSidecar`/`safeName`), plus robustness
  (never panic/hang) of the parsers (`dateparse`, `items.Classify`,
  `ensemble.Reconcile`, `parseICS`, the vsock/broker HTTP framing).
- Does NOT fit the semantic marker attacks (prompt-injection surviving a guard,
  jailbreaks) — no syntactic oracle; leave those as seed-based ADD cases.
- **API:** `add.FuzzSanitizer(f, seeds, transform, badRe)` — `badRe` must never
  appear in `transform(input)`; `add.FuzzInvariant(f, seeds, func(t,in))` —
  generic (robustness / custom property). Fuzz targets are `FuzzXxx(f *testing.F)`
  in the consumer package, one line each.
- **The loop closes itself:** a fuzz-found failure is saved to `testdata/fuzz`
  and replays deterministically in plain `go test` (CI gets it free). Promote the
  saved input to an ADD seed → discovery becomes permanent regression.
- **CI:** fuzz-search is time-bounded + nondeterministic — a separate longer job
  (`-fuzztime`); the saved corpus runs in the fast gate. Keep targets pure, fast,
  no network/VM.

## Non-goals / anti-friction rules
- Go only. Lean on `go test -json`, `testing.TB`, subtests, `-run`.
- No network/black-box adapters in v1 (white-box, in-process).
- No severity scoring, ticketing, or dashboards in the library — emit JSON, let
  something else render it.
- Keep the core under a few hundred lines. If it grows, the oracle/coverage
  parts are where complexity is allowed; the runner is not.

## Reliability cases — AI-engineering (both hats, docs/COURSE-COVERAGE.md)
`add.Technique`s for the reliability track (blog Posts 25–28). Invariant: undefended
ASR 100% → defended 0%.
- ✅ `ResumeIntoTamperedState` (durable execution, ASI10): a forged/edited
  checkpoint is replayed on resume. Defend: `durable.Open`/`Verify` recompute the
  hash-chain and fail closed on a tampered record. Shipped (`durable` pkg + redteam).
- ✅ `DuplicateSideEffect` (exactly-once, ASI08): a replayed/duplicated action
  fires the side effect twice. Defend: `durable.Do` idempotency-key dedup (first
  success cached, failure retryable). Shipped. Follow-on: version preconditions.
- ✅ `SummarizationInjection` (context compaction, LLM01): an injected instruction
  gets laundered into the running summary. Defend: `compaction.Compactor`
  provenance partition — untrusted turns summarized separately, kept Trusted=false,
  guard.Sanitize'd, never promoted to trusted context. Shipped (`compaction` pkg).
- `JudgeManipulation` (LLM-as-judge, model-blocked): prompt-inject the judge to
  pass a bad output. Defend: judge-input encapsulation + a deterministic
  cross-check (the ensemble pattern). Needs the live LLM path (C5/C7).
