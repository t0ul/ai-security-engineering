# Control-plane hardening plan — the governed runtime

Goal: a control plane where **every knob that changes the agent's behavior is
resolved at runtime from one governed store**, and each knob is versioned,
content-hashed, changed only through dual-control approval, **gated by eval F1 +
ADD ASR**, pin-verified at use, rolled back as a known-good bundle, and audited.
Today we have the *mechanism* (goverlord propose/approve/rollback, gledger audit,
cpstore inventory) but the agent barely reads from it — most behavior is consts,
struct fields, and dead storage stubs. This plan closes that.

Implementation is t's (dev-caveman); this is the architecture.

## Audit — governed knob inventory (code-verified)
| Knob | Where it lives now | Governed at runtime? | In console? | Persisted/audited? |
| --- | --- | --- | --- | --- |
| **Prompts** (planner/coder/extractor) | consts; `controlplane.Prompts` resolver (C1) | orchestrator ✅, extractor ❌ | ❌ | hook exists, not wired |
| **Sampling** (temperature/max_tokens/stop/top_p/seed) | hardcoded `CompleteOpts` literals | ❌ | ❌ | ❌ |
| **Model bindings** (logical→gguf, allowlist) | orchestrator fields; gouncer allowlist | ❌ (fields); no `extractor` | ❌ | ❌ |
| **MCP registry** (servers, allowed tools, manifest pins, strict) | `gustoms` (Pin/Approve/ManifestHash) | enforced at Call ✅ | ❌ no view | **`RecordPin` is dead code — never called** |
| **Guardrails** (gumpers rails, topic denylist, thresholds) | consts/flags | ❌ | ❌ | ❌ |
| **Egress/exec policy** (netpolicy allowlist, argv allowlist, action-link allowlist) | flags/consts | ❌ | ❌ | ❌ |
| **Budgets/limits** (rate, token, concurrency, per-caller) | gouncer gateway | gateway-side, not plane | ❌ | ❌ |
| **Keys/virtual keys** | gouncer | gateway-side | ❌ | ❌ (secrets stay out of config; pointers only) |
| **Eval scores + thresholds** (F1 gate, ASR gate, per-label latest) | `eval.ScoreEvents`; `cmd/livecheck`→`RecordEval`; `cpstore.LatestEval`; `mlops.PromoteWithLatestEval` | standalone; not tied to changes | ❌ | partial (livecheck only) |
| **Promotion gate** (eval+ADD on activate) | `controlplane.PromotionGate` (C3) | not wired to real closures | ❌ | — |
| **Safety level / kill switch** | `Governance.SetKillSwitch`, `Safety` | ✅ | ✅ | ✅ |
| **RBAC / four-eyes** | goverlord roles + dual control | ✅ | partial | ✅ |
| **Generic config map** | `Governance` (propose/approve/rollback/history) | ✅ mechanism, **~nothing reads it** | ✅ | ✅ (approvals, admin) |

**The core defect:** the governed `Config()` map is disconnected from the runtime.
The fix is a **single resolver** the whole agent reads behavior from, backed by
the governed store, with the shipped const as the fail-closed default.

## Principle — one governed runtime config, resolved everywhere
Introduce `controlplane.Runtime` (extends the C1 `Prompts` idea to all knobs): a
typed view over the governed `Config()` + `cpstore`, with safe defaults. The
agent calls `rt.Prompt("planner")`, `rt.Sampling("planner")`,
`rt.Model("extractor")`, `rt.Policy("egress")`, `rt.MCP()` — never a const or a
struct field. A governed change takes effect with **no redeploy**; an un-set knob
resolves to the shipped default (fail-closed). Every resolve can pin-verify
(content hash) and fail closed to the last known-good on mismatch (ties to AIDR).

## The eval-tweak loop (the explicit ask: tweak prompts and test)
A prompt-engineering workbench with guardrails:
1. Operator proposes a prompt (or sampling/model) change — dual-control.
2. The plane runs the candidate through **both oracles on a shadow run**: the
   eval harness (F1 vs the labeled ground-truth) and the ADD suite (ASR).
3. `PromotionGate.Allow()` promotes ONLY if F1 ≥ current baseline AND ASR = 0.
4. On pass → activate (new version), `RecordPrompt` + `RecordEval` + gledger
   audit. On fail → keep known-good, show the F1 delta + which ADD case broke.
5. Console shows: current prompt per model, its F1/ASR, a **diff**, and a
   **"Test"** button that runs the shadow eval and displays the delta *before*
   you promote. This is how you tweak prompts safely.

## MCP governance (list / pins / re-approve) — currently missing
- **Persist pins:** call the dead `cpstore.RecordPin` on `gustoms.Approve` +
  gledger audit, so the approved manifest hash is durable and attributable.
- **Registry as governed config:** servers, `AllowedTools`, strict-pinning,
  authorizers live in the governed store (propose/approve to add a server or a
  tool), not hardcoded in `cmd/mcpdemo`.
- **Console MCP tab:** list servers → their advertised tools → pin status
  (approved hash vs current), a **rug-pull alert** when the manifest changed, and
  an **Approve/re-approve** button (dual-control). Surface `ErrPinMismatch` /
  `ErrNotApproved` as operator actions.
- **Brokered scoped tokens** (M20): per-call audience-bound tokens via captoken,
  governed TTL/scope.

## Sampling governance
- Move `CompleteOpts` to `rt.Sampling(model)` → `{temperature, max_tokens, top_p,
  stop, seed}` per logical model, from the governed store (defaults = today's
  literals). Tunable + versioned + part of the eval-tweak loop (a temp change is
  gated like a prompt change).
- `seed` governed → **reproducible** generations for eval/forensics.

## Model-swap plane
- Logical `extractor` model (gouncer-routed) so extraction uses a different
  physical model than the planner — swap = config edit, not code.
- `response_format: json_schema` / GBNF grammar on extraction (valid-by-
  construction JSON).
- A model swap is a governed change gated by the promotion gate (eval + ADD).
- Candidate: 7–8B extractor vs the 3B planner — decided by the gates, 16GB RAM
  trade-off (one hot extraction model + on-demand coder).

## Ensemble + ADD (done)
`agent/ensemble.Reconcile` (LLM extract / `dateparse` verifies dates M9 / cross-
check / hallucination-reject) ✅. ADD: `ExtractionInjection` (LLM01) +
`HallucinationReconcile` (LLM09) ✅, in the suite 100→0. These gate every
prompt/model swap.

## Console surfaces (operator can actually govern the agent)
Tabs the console must add beyond today's generic config/history/approvals:
**Prompts** (per model: active version, hash, F1/ASR, diff, Test, propose,
rollback) · **Models/Sampling** (bindings + temp/tokens, gated edit) · **MCP**
(servers/tools/pins, rug-pull alerts, re-approve) · **Policies** (egress/argv/
guardrail allowlists, gated edit) · **Eval** (F1/P/R per label over time, ASR
trend) · **Budgets** (rate/token/spend) · existing **Governance/Audit/Kill**.

## Phases (t implements; each offline-testable unless noted)
- **C1 — Prompt resolver** ✅ (orchestrator). `controlplane.Prompts`
  (Get/Activate/Verify, versioned+hashed, OnActivate hook). Extractor → C5.
- **C2 — Governed prompt lifecycle + console**: propose→approve→rollback a prompt
  through `Governance`; wire `OnActivate`→`RecordPrompt`+gledger; console Prompts
  tab (diff/Test/approve/rollback).
- **C3 — Promotion gate + ADD** ✅: `PromotionGate` (eval-F1 + ADD-ASR, fail-
  closed) + `ActivatePrompt`; `ExtractionInjection` + `HallucinationReconcile`
  ADD cases; `agent/ensemble` reconciler. Remaining: wire the REAL eval/redteam
  closures into the gate (needs a cmd that imports both, to avoid a cycle).
- **C4 — `controlplane.Runtime`**: one typed resolver over the governed store for
  prompts + sampling + policies + model bindings, with pin-verify + fail-closed
  defaults; make the agent read ALL knobs from it.
- **C5 — Sampling + model-swap plane**: `rt.Sampling`, logical `extractor`,
  grammar-constrained JSON; sampling/model changes gated.
- **C6 — MCP governance**: persist pins (`RecordPin`), registry-as-config,
  console MCP tab (list/pins/rug-pull/re-approve).
- **C7 — Eval-tweak loop**: shadow eval on a candidate, F1/ASR delta surfaced,
  gate on promote; console Test button + eval history.
- **C8 — Policies as governed artifacts**: egress/argv/guardrail allowlists into
  the governed store with the same lifecycle.
- **C9 — Budgets/limits + key pointers**: rate/token/concurrency/spend governed;
  keys referenced by pointer (never stored in config).
- **C10 — Known-good bundle**: atomic snapshot/rollback across ALL knob classes;
  console "roll back to <version>" for the whole plane.

## Course mapping
M14 (governed control plane + RBAC + four-eyes) · M15 (admin audit + versioned
rollback + known-good bundle) · M19 (promotion gates, model/data governance) ·
M20 (gateway-as-product: virtual keys, budgets, DLP; MCP registry/pins/brokered
tokens) · M9/M11 (eval + ADD gates; the tweak-and-test loop). This is Track II's
real completion: the plane governs prompts, sampling, models, MCP, and policies —
not a generic map nothing reads.
