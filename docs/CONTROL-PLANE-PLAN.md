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
| **Prompts** (planner/coder/extractor) | consts; `controlplane.Prompts` resolver (C1) + `GovernedPrompts` in `cmd/webapp` | orchestrator ✅; extractor resolves when LLM path enabled (C5) | **console Prompts tab ✅ (C2)** — list/version/hash/activate/reset | **`RecordPrompt` wired on the app path ✅** + gledger audit |
| **Sampling** (temperature/max_tokens/stop/top_p/seed) | hardcoded `CompleteOpts` literals | ❌ | ❌ | ❌ |
| **Model bindings** (logical→gguf, allowlist) | orchestrator fields; gouncer allowlist | ❌ (fields); no `extractor` | ❌ | ❌ |
| **MCP registry** (servers, allowed tools, manifest pins, strict) | `gustoms` (Pin/Approve/ManifestHash/**Status**) on the app path via `NewToolGateway` | enforced at Call ✅ | **console MCP tab ✅ (C6)** — list/tools/pin/rug-pull/Approve | `RecordPin` wired in `cmd/mcpdemo`; app gateway pin is deterministic from the shipped manifest |
| **Guardrails** (gumpers rails, topic denylist, thresholds) | consts/flags | ❌ | ❌ | ❌ |
| **Egress/exec policy** (netpolicy allowlist, argv allowlist, action-link allowlist) | `controlplane.Policies` resolver + `GovernedPolicies` in cmd/webapp | egress resolves live at action time ✅ (C8); exec allowlist governed artifact | **console Policies tab ✅ (C8)** — list/version/hash/activate/reset | `RecordPolicy` + gledger audit ✅ |
| **Budgets/limits** (rate, token, concurrency, per-caller) | gouncer gateway | gateway-side, not plane | ❌ | ❌ |
| **Keys/virtual keys** | gouncer | gateway-side | ❌ | ❌ (secrets stay out of config; pointers only) |
| **Eval scores + thresholds** (F1 gate, ASR gate, per-label latest) | `eval.ScoreEvents`; `cmd/livecheck`→`RecordEval`; `cpstore.LatestEval`/`ListEvals` | **console Eval tab + Run + shadow-eval Test (C7)** ✅ | **Eval tab ✅** | persisted + shown |
| **Promotion gate** (eval+ADD on activate) | `controlplane.PromotionGate` (C3) | not wired to real closures | ❌ | — |
| **Safety level / kill switch** | `Governance.SetKillSwitch`, `Safety` | ✅ | ✅ | ✅ |
| **Identity / authZ (NHI)** | `controlplane.Authority` + capability `Grant` (C4a) + `webapp` authz gate (C4b) + tool NHIs + residency policy + MCP gateway (C4e) — **C4 complete** | on the API path ✅ + each tool is a scoped NHI; MCP (gustoms) on the app path ✅ | ❌ console tab (C6) | denials audited to gledger ✅ |
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

## Non-human identity & capability authZ — the C4 keystone
The app is being built as an **authZ'd API**, not just a loopback parse tool, so
it can teach **non-human identity (NHI)** and **MCP security** and make the kill
switch a real containment demo. Every actor that is not a human — the agent, each
tool, each MCP server, the RAG reader — is an NHI: a named Subject that must
present a signed, scoped, short-lived **capability `Grant`** to do anything with a
side effect. Grants reuse the agent's existing ed25519 provenance signer, so an
NHI credential and an `.ics` content credential share one trust root — no new key
management. (`controlplane.Authority` / `Grant` / `Capability`, C4a ✅.)

**Scoping is data-residency-aware — three axes, not one:**
1. **Action granularity** — `read` (one named item) ≠ `list`/`enumerate` (the
   whole set — the *harvest* primitive) ≠ `export`/`send` (egress). Exfil needs
   enumerate + export, so those stay privileged apart from read. `/list/contacts`
   is dangerous because it is the harvest, not a read.
2. **Resource classification** — public (handbook) / internal / confidential-PII
   (contacts, raw email bodies, student data).
3. **Consumer trust tier** — where the Subject's *bound model* runs: on-host-local
   (data never leaves the Mac) vs off-host-frontier (data ships to a third party).
   The same read scope that is safe for a local model is an exfil vector for a
   frontier one.

**The rule (at issuance, fail-closed — `Authority.IssuePolicy`):** a grant is
mintable only if the consumer tier is cleared for the resource classification. A
frontier-bound Subject is denied any confidential resource and any `list`/`export`
over one; it may receive only public or **goflage-scrubbed (declassified)** data.
goflage is the declassifier at the tier boundary. Corollary: swapping a Subject's
model binding local→frontier (a governed C5 change) is a **grant-invalidating
event** — the swap must `Revoke` the Subject so no outstanding grant silently
starts leaking. You cannot raise data egress by editing a model binding.

**Kill switch with teeth:** `Authority.Halted` wired to `Safety` ≥ `LevelHalt`
revokes *every* grant at once. An in-flight action (a VM fetch mid-request) dies
on its next capability check, not just new drops paused. The demo: fire an action
link, hit kill, watch the token rejected mid-flight.

**Honest caveats (no theater):** single-household loopback app — do **not** fake
human SSO/login forms; the interesting identities here are non-human. Human
operator = one minimal local admin credential; loopback stays the perimeter.
SQLite has no DB roles, so RAG write-denial is engine-enforced (`?mode=ro`) but
tenant scoping is app-layer — label which is which. Auth is additive: drop a
`.txt`, it still just works.

**ADD cases (red-team the identity layer), each `undefended 100%→defended 0%`:**
`StolenAgentToken` (replay/exfil → bound to scope+TTL+key), `OverScopedToken`
(confused deputy → `ErrGrantScope`), `RevokedTokenStillWorks` (kill engaged,
in-flight grant honored → must fail), `FrontierContactsExfil` (frontier identity
mints `list contacts` → refused at issuance / revoked on swap), `CrossTenantRead`
(RAG query leaks another tenant → denied), `UnpinnedMCPServer` (swapped manifest
hash → rejected by gustoms pin). Each also gets an ADD `Wired` integration target
so the bridge can't silently un-wire.

## Phases (t implements; each offline-testable unless noted)
- **C1 — Prompt resolver** ✅ (orchestrator). `controlplane.Prompts`
  (Get/Activate/Verify, versioned+hashed, OnActivate hook). Extractor → C5.
- **C2 — Governed prompt lifecycle + console** ✅: `Prompts.List`/`Reset` +
  `GovernedPrompts` wired in `cmd/webapp` over a cpstore inventory, so activations
  are versioned, content-hashed, audited to gledger, and persisted
  (`RecordPrompt` — previously dead — now fires on the app path). Console Prompts
  tab (Security-adjacent): per-model active version + hash, edit→**Activate new
  version**, **Reset to default** (rollback to shipped known-good). Live smoke:
  list→activate v1→reset v0, 401 without a capability, inventory.db persisted.
  Honest scope: single-operator loopback (no four-eyes — that's Track II
  `Governance`); the **Test** (shadow-eval) button is C7; version-N text rollback
  (vs reset-to-default) waits on reading historical text back from cpstore.
- **C3 — Promotion gate + ADD** ✅: `PromotionGate` (eval-F1 + ADD-ASR, fail-
  closed) + `ActivatePrompt`; `ExtractionInjection` + `HallucinationReconcile`
  ADD cases; `agent/ensemble` reconciler. Remaining: wire the REAL eval/redteam
  closures into the gate (needs a cmd that imports both, to avoid a cycle).
- **C4 — NHI identity + capability authZ (keystone, offline)**: the app becomes
  an authZ'd API where every non-human actor presents a scoped, signed, expiring
  capability `Grant`. Also carries the C1 resolver idea forward — the governed
  `Runtime` that issues/scopes credentials is the same plane that resolves
  prompts/sampling/policies. Sub-phases:
  - **C4a — capability token** ✅: `controlplane.Authority`/`Grant`/`Capability`
    on the provenance signer — issue (TTL + scope), verify (sig → halt/revoke →
    expiry → scope, fail-closed), `Revoke`/`Reinstate`, `Halted` hook, action
    granularity (`ActionRead`/`List`/`Write`/`Export`), data-residency
    `IssuePolicy`. Unit-tested incl. the frontier `/list/contacts` refusal.
  - **C4b — authZ gate on the app** ✅: `Server.authz(action,resource)` verifies a
    bearer capability `Grant` on the mutating/listing endpoints — `/api/ask`
    (list/corpus), `/api/accept` (write/calendar), `/api/drop` (write/inbox),
    `/api/action` (export/link) — fail-closed, denials audited to gledger. The
    operator holds one broad grant minted from the agent key (`webapp.EncodeToken`),
    injected into the page; a page fetch wrapper attaches it to every /api+/ics
    request. Gate is a no-op when `Authz` unset (household mode). The kill switch
    stays OUTSIDE the gate (Halt revokes all grants, so a gated killswitch could
    never disengage). httptest covers missing/valid/over-scoped/halted; live smoke
    confirmed (no token 401, token passes). Remaining: narrower per-tool grants
    land with C4d/e.
  - **C4c — kill switch = revocation** ✅: `Authority.Halted` wired to `Safety` ≥
    `LevelHalt` (cmd/webapp) — Halt revokes every grant so in-flight HTTP side
    effects die (`TestAuthzHaltRevokesInFlight`), locked as the `RevokedTokenStillWorks`
    ADD invariant (`redteam`, ASI10, 100→0). Remaining nit: drop MCP pins on Halt —
    rolls into C4e when gustoms lands on the app path.
  - **C4d — RAG least privilege** ✅: `rag.OpenReadOnly` opens the corpus `?mode=ro`
    (engine-enforced no-write) for the Ask/retrieval path — a separate handle from
    the writable ingestion one; tenant ACL is app-layer in `Query`'s WHERE
    (authorization-first). `rag.TestOpenReadOnlyCannotWrite` + the existing
    `RAGTenantLeak` (CrossTenantRead) ADD case. cmd/webapp wires the RO reader;
    live smoke green.
  - **C4e — scoped tool NHIs + MCP on the app path** ✅: (1) the action-fetch tool
    is its own identity (`action-fetcher`, export/link grant) and the RAG reader
    its own (`rag-reader`, list/corpus grant), each verified per call so Halt
    revokes them in-flight; (2) `controlplane.ResidencyPolicy` wired as the
    `IssuePolicy` (frontier-bound subject refused list/export of confidential data;
    corpus is declassified by goflage, so it is allowed — swap-ready for C5); (3)
    the fetch tool is reached through a gustoms gateway (`controlplane.NewToolGateway`:
    manifest-pinned, allow-listed, authorizer denies all calls while the kill switch
    blocks tools = functional "drop pins on halt"). Tests: `TestResidencyPolicy`,
    `TestToolGateway*`; `McpRugPull` already covers the pin-mismatch defense the
    app-path gateway enforces; live smoke green. **C4 keystone complete (C4a–C4e).**
- **C5 — Sampling + model-swap plane** 🟡 (live-doable now — models verified up
  this session; **the LLM extractor already works**: `EXTRACT_MODE=llm` scores
  F1=1.00/1.00 live via `cmd/livecheck` + `livetest`). Remaining: a **logical
  `extractor` model binding** (route extraction to its own gouncer model, swap =
  config not code), **sampling governance** (`rt.Sampling(model)` → temp/top_p/
  stop/seed from the governed store; `seed` → reproducible gens), and
  **grammar-constrained JSON** (GBNF / response_format on extraction, valid-by-
  construction). A model/sampling swap is gated by the promotion gate (eval F1 +
  ADD ASR) and, per the residency policy (C4e), re-evaluates outstanding grants.
- **C6 — MCP governance console** ✅: `gustoms.Gateway.Status` (read-only snapshot:
  tools, allow-list, approved pin vs live manifest, mismatch) + `webapp` MCP card
  in the Security tab (status pinned/rug-pull/unapproved/blocked/error) + operator
  **Approve** (re-pin, csrf + authz write/mcp). Live smoke: pinned→blocked under
  the kill switch; approve 200 with token / 401 without. Remaining (not blocking):
  registry-as-governed-config (add servers via propose/approve) when a 2nd MCP
  server exists; durable `RecordPin` for the app gateway (its pin is deterministic
  from the shipped manifest, so not needed for the single in-process tool).
- **C7 — Eval-tweak loop** 🟡→mostly ✅: shipped the console surface — an **Eval
  tab** (`/api/eval` lists persisted F1 from `cpstore.ListEvals`; **Run eval now**
  = `/api/eval/run` runs `eval.Score` live and records it) and a **"Test" button**
  on each prompt (`/api/prompts/test` = shadow eval: set the candidate as the live
  extractor prompt via `extractor.SetExtractionPrompt`, run the eval + the ADD
  suite, and return F1 vs baseline + ASR + the `PromotionGate` verdict **without
  activating**). The extractor now honors the governed/candidate prompt
  (`ActivePrompt()`), and activating the "extractor" prompt drives extraction (a C5
  down payment). Honest edges: the webapp doesn't run gouncer, so Run/Test are live
  only when a gateway is up at :4000 (else regex fallback, clearly messaged); shadow
  eval applies to the `extractor` prompt (planner/coder drive the orchestrator).
  Remaining: an **ASR trend** line + embedding/pointing-at a gateway so the console
  Run is live without a separate `livecheck`.
- **C8 — Policies as governed artifacts** ✅: `controlplane.Policies` resolver
  (`List`/`Activate`/`Reset`/`Verify`, order-independent `HashPolicy`) +
  `GovernedPolicies` over the cpstore inventory (`RecordPolicy` + gledger audit).
  Seeded with `egress` (action-link hosts) and `exec` (argv[0] allowlist). The
  webapp action egress check resolves its allowlist **live** from the governed
  policy (`Server.egressPolicy`), so a widening/rollback takes effect with no
  redeploy — and deny-by-default (private/IMDS, argcheck) is unaffected, so the
  SSRF / AllowlistBypass ADD cases still hold. Console Policies tab (list / version
  / hash / activate / reset). Live smoke: activate v1 → reset v0, 401 without a
  capability, 2 rows persisted + audited. No new ADD case: C8 governs *which*
  allowlist is active (a lifecycle/audit property, unit-tested); the enforcement
  it feeds is already covered by `SSRF`/`ActionLinkExfil` (egress) and
  `AllowlistBypass` (exec).
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
