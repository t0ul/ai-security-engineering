# Integration gaps — library-proven vs wired-into-the-app

Honest ledger (code-verified) of controls that pass their ADD/unit tests but are
NOT on the real app's path (`cmd/webapp` → watcher → pipeline → webapp.Server).
These were marked "✅ shipped" meaning *the library exists and its ADD case is
green* — not *the running app uses it*. The ADD `Wired` (integration) target was
designed to catch exactly this and hasn't been used yet.

The app's actual imports: `agent/{pipeline,roster,tool,watcher}`, `controlplane`
(only `SandboxFetchTool` + `Interpreter` for action fetches), `netpolicy`, `rag`,
`webapp`.

## HIGH — correctness/privacy, in the app the user runs
- ✅ **FIXED — PII scrub (goflage) now runs before corpus index.** Pipeline +
  `cmd/reindex` scrub with `goflage.Scrub` before `rag.Add`; audited as
  `pii/scrubbed`. goflage SECRET_KEY recognizer extended to `:`-form +
  username/login labels with intervening chars. Verified live on email 9: both
  credential lines caught, **0 creds in corpus**. Raw backup stays in processed/.
  Test: `TestCorpusIndexIsScrubbed`.
- **Prompt resolver (C1) is not set in `cmd/webapp`.** The resolver exists and the
  orchestrator reads it — but the app doesn't run the orchestrator, and nothing
  constructs/activates a `Prompts`. Until wired + an extractor resolver exists,
  the app still uses consts. (My own C1 is a facade in the app until C4/C5 wire it.)

## MED — "governed agent" story that isn't on the app path
All library-proven (ADD green) but 0 uses in the app:
- **CaMeL control plane** (`controlplane.Orchestrator`): the email agent is
  single-LLM/regex on the host; the planner/coder/sandbox loop runs only in
  `cmd/controlplane` (demo).
- **Governance / RBAC / four-eyes**: the app has no governed config or operator
  approval flow. Demo: `cmd/console`, `cmd/gridge`. ✅ **Kill switch now wired**:
  `controlplane.Safety` in `cmd/webapp` — Pause halts pipeline processing + drops
  (`pipeline.Halted`), BlockTools refuses accept/action; `/api/killswitch` +
  `/api/safety` + UI control. Tests `TestKillSwitchGatesSideEffects`,
  `TestHaltedPipelineSkips`. (aidr auto-contain is contrived for a parse-only app
  — manual switch only; follow-up if the app gains executing tools.)
- **MCP gateway** (`gustoms`): pinning/allowlist/authz run only in `cmd/mcpdemo`.
  The app makes no MCP calls.
- **A2A signing** (planner→executor): orchestrator has Signer/Verifier fields;
  the app never sets them (and never runs the orchestrator).
- ✅ **FIXED — Provenance signing (M20) wired into the app.** `cmd/webapp` loads/
  creates a persistent ed25519 agent key (`<drop>/agent.key`, 0600), sets
  `pipeline.Signer` → every emitted `.ics` gets a `.sig`; `webapp.Server.Verifier`
  checks it and `/api/events` reports `signed`, shown as a "✓ signed" badge.
  Integration test `TestEventsReportProvenance` (signed→true, tampered→false).
  New `provenance.SignerFromSeed` for the persistent identity.
- ✅ **FIXED — HITL now uses the `hitl` evidence-first, single-use nonce confirm**
  (ASI09). `/api/accept` + `/api/action` are two-phase: phase 1 returns the
  evidence + a one-time nonce (no side effect); phase 2 performs only on the
  correct echoed nonce for that exact action. A forged/replayed nonce is 403.
  UI shows an evidence dialog. Tests: `TestAcceptRequiresHITLConfirm`,
  `TestAcceptForgedNonceRefused`, action two-phase.
- **aidr** (detect→contain → auto kill switch), **quorum** (M-of-N),
  **captoken** (scoped per-call creds), **memory** (TTL/scope): ADD-only.

## Dead storage stubs — ✅ FIXED
- ✅ `cpstore.RecordPin` now has a live caller via `gustoms.WithPinRecorder`
  (called on `Approve`), wired in `cmd/mcpdemo`; `cpstore.ListPins` getter added.
  Tests: `TestWithPinRecorderOnApprove` (gustoms), `TestPinsPersistAndList`.
- ✅ `cpstore.RecordPrompt` now called via `controlplane.GovernedPrompts`
  (`OnActivate` → RecordPrompt + gledger audit); `cpstore.ListPrompts` getter
  added. Tests: `TestGovernedPromptsPersist`, `TestPromptsPersistAndList`.
- `RecordEval` / `RecordAdmin` / `LatestEval` — 1 caller each (livecheck /
  governance); thin but live.

## Already tracked elsewhere (not re-litigated here)
- Sampling params hardcoded; governed `Config()` is a facade; MicroVM not used by
  the agent (only the action-fetch path, VM must be booted) — see
  `docs/CONTROL-PLANE-PLAN.md` and `docs/MICROVM-PLAN.md`.

## Live bugs in WIRED code — ✅ FIXED this pass
- ✅ **`/ics/` now serves ONLY `.ics`** (bare basename, suffix-checked);
  `*.summary.json` sidecars 404. Test: `TestICSServesOnlyICS`.
- ✅ **CSRF/Origin check** on `/api/drop|accept|action`: a foreign `Origin` is
  refused (403); same-origin/loopback passes. Test: `TestCrossOriginPostRefused`.
- ✅ **Year default = `time.Now().Year()`** in both cmd mains (no more 2026
  time-bomb). *Remaining nicety ⬜:* next-year rollover for a cross-year undated
  date (Dec email about a Jan event) — `dateparse` enhancement.
- ✅ **`ics.uid()`** falls back to a time-based id on a `rand` error (no all-zero
  UID).

Honest positives this pass: `gledger.Emit` IS mutex-protected (concurrent audit
writes safe); `os.WriteFile` errors in drop/accept ARE handled.

## Honest positives (actually wired into the app ✅)
`watcher`→`pipeline`; `guard` injection-sanitize + `gumpers` (via guard); the ICS
field sanitizer; `netpolicy` on action egress; `rag` corpus; `gledger` audit
(incidents/activity read it via `ir`); the items extractor + dedup; SandboxFetch
for actions (when the VM is up).

## What this means
Two worlds today: (A) the email→calendar **product** (webapp + pipeline + items),
and (B) the security-course **controls** (library + ADD + demos). The ask — "make
the control plane robust for the app" — is the bridge: wire B into A. Each bridge
is a small integration slice + an ADD `Wired` (integration) target so it can't
silently un-wire again. Priority: goflage-on-ingest (HIGH), then the governed
runtime (C4) carrying prompts/sampling/policies into the app, then HITL dialog,
provenance signing, and MCP (if the app gains tool use).
