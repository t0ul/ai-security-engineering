# Platform Backlog — intended scope vs built

Reconciles the skeleton draft (`scratch/skeleton-draft.md`) and the syllabus
(`scratch/syllabus.md`) against what the Go fold-in actually built. Legend:
✅ built & tested · 🟡 partial · ⬜ not built.

## Track I — build & harden
| Module | Capability | Status |
| --- | --- | --- |
| M0 | threat model / data-flow / trust boundaries | ⬜ |
| M1 | MicroVM detonation sandbox (vsock, detonationd) | ✅ |
| M2 | gateway (gouncer) + hash-chained audit (gledger) | ✅ |
| M3 | PII/secret scrub (goflage) | ✅ |
| M4 | injection + system-prompt leak (gumpers/guard) | ✅ |
| M5 | output handling / exfil | ✅ SSRF/web-fetch lab + hardened sanitizer (inline+reference images, raw `<img>`, autolinks, `js:`/`data:`/`vbscript:`/`file:`); .ics field sanitizer + RFC5545 escape |
| M6 | excessive agency / HITL / circuit breaker | ✅ |
| M7 | MCP & A2A security (gustoms) | ✅ gustoms gateway + mcp transport + agent/a2a (wiring a2a into live loop pending) |
| M8 | RAG / KB security | ✅ `rag` on SQLite: FTS5 lexical + semantic (cosine over embeddings, `HTTPEmbedder`→gouncer), tenant-ACL-as-WHERE on both paths, provenance, sanitized recall; embedding-inversion defended (vectors server-side + ACL); poisoning/tenant-leak ADD |
| M9 | data integrity / misinformation | ✅ weekday-integrity dateparse + cross-email conflict detector (`agent/tools`) + confidence scoring & `Event.NeedsReview()` HITL flag |
| M10 | supply chain / model artifacts | ✅ `assets.CheckModelFormat` (reject pickle) + gguf magic/sha + safetensors header verify; dep-SCA tracked |
| M11 | eval harness + red-team automation (gorauder ASR) | ✅ |
| M12 | full-chain capstone | ✅ `cmd/scorecard` (15 techniques ASR, CI gate) + `ir`/`cmd/replay` (reconstruct an incident by trace_id from the chain-verified log) |
| M13 | productionization / deployment | 🟡 runbook + static binaries; egress-denied MicroVM + allowlist/argcheck + non-root + ephemeral + `sandbox_exec` MCP tool landed (live VM re-boot pending); checklist + health/degraded modes missing |

## Track II — operate & govern
| Module | Capability | Status |
| --- | --- | --- |
| M14 | governed control plane + operator RBAC | ✅ (generic config); 🟡 prompts/models/policies not yet governed artifacts — see `docs/CONTROL-PLANE-PLAN.md` (prompts management plane, model-swap plane, promotion gates) |
| M15 | admin audit + versioned rollback | ✅ |
| M16 | network/credential containment (egress allowlist, DNS pinning, block link-local/RFC1918, SSRF→IMDS) | ✅ `netpolicy` + sandbox exec hardened to `argv` allowlist + `argcheck` arg-arrays (no shell strings); no-ambient-creds (captoken) exercised in redteam, live-loop wiring open |
| M17 | AppSec/supply-chain of the harness (SAST/DAST/SCA, SLSA, AISBOM, signed images) | ⬜ **SKIPPED (operator's call)** |
| M18 | IR: kill switch + IR replay + AIDR + playbooks + degraded + PIR→CI | 🟡 kill switch ✅, IR replay ✅, AIDR auto-trigger ✅ (`aidr`); IR playbooks + degraded mode + PIR→CI pending |
| M19 | MLOps/data/privacy | ✅ `registry` promotion gates (+eval-gated via cpstore) · `memory.Erase` DSAR · `dataset` provenance verify at ingest; post-fine-tune/RLHF N/A (no training pipeline) |
| M20 | memory lifecycle + output authenticity + MCP gateway | ✅ `memory` (TTL/scope/erasure/untrusted-recall) + `provenance` (ed25519 content credentials) + MCP gateway (gustoms) |
| Capstone II | operator console (local web app) | ✅ `controlplane.ConsoleServer` + `cmd/gridge`: served page + JSON API over the governed plane, reused in tests. Wails dropped — local web app reused for tests (decision) |
| App (owner use) | beyond meetings: tasks/reminders, heads-ups, actions, digest+contacts surfacing, needs-review, month view | ⬜ planned — `docs/APP-FEATURES-PLAN.md` (item-kind model, VTODO, summary sidecars, new endpoints/tabs; phases A1–A8) |

## Specific gaps the operator flagged
- **Layered kill switch** — spec (skeleton §28): revoke tokens → gateway fail-closed → scoped levels (pause-sessions / block-tools / full-halt) → AIDR auto-trigger → drain+snapshot. Today: binary `goverlord.KillSwitch`.
- **MCP server pinning / rug-pulls** — spec (skeleton §102, §106): registry allowlist of approved servers, content-hash pin, re-approve on change, peer tool authZ at call time, brokered scoped audience-bound tokens. Today: nothing; `gustoms` unbuilt.

## Tool roster (syllabus "One Agent, Many Scoped Tools") — status
Each tool is a drop-in behind the `agent/tool` plugin contract (name, typed
output, one capability, own labels). Demonstrates least-privilege per tool.
| # | Tool | Capability | Status |
| --- | --- | --- | --- |
| 1 | Event → .ics extractor | write_ics | ✅ |
| 2 | Action-items / deadlines | read_only | ✅ `agent/tools` |
| 3 | Digest / TL;DR | read_only | ✅ `agent/tools` |
| 4 | Contacts extractor | read_only (PII-scoped) | ✅ `agent/tools` |
| 5 | Change / conflict detector | read_only | ✅ `agent/tools` (also M9) |

## Other agentic-security items still owed (from the skeleton)
- **A2A security (M7)** — authenticate/validate/attenuate inter-agent messages
  (planner↔executor↔coder today trust each other implicitly); per-agent identity,
  message signing, attenuated scoped tokens.
- Multi-agent poisoning/collusion; runaway-orchestration budgets (beyond the
  step breaker); human-automation-bias / clickjack-resistant approval UX;
  canary/honey-token exfil tests; side-channels; deceptive-model/backdoor checks.

## Merged build plan (this effort); skip M17
- **A — MCP pillar + containment (M7 + M16 + M5-SSRF).** ✅ DONE: `gustoms` MCP
  gateway (allowlist, manifest pin / rug-pull defense, per-tool authZ, audit);
  `mcp` transport + `web_fetch` tool; `netpolicy` egress guard (default-deny,
  block loopback/RFC1918/link-local+IMDS, DNS-rebinding); `cmd/mcpdemo` (7
  scenarios green, audit verifies). Tested.
- **B — tool roster (tools 2–5).** ✅ DONE: `agent/tools` — action_items,
  digest, contacts (PII-scoped via goflage), conflict_detector; all read-only,
  capability-enforced, tested.
- **C — A2A security (M7/ASI07).** ✅ DONE: `agent/a2a` HMAC-signed messages + **wired into the live loop** (planner signs plan, executor rejects unauthenticated hand-off; tested).
- **D — layered kill switch (M18 core).** ✅ DONE: `controlplane/safety.go` —
  levels none/block-tools/pause/halt; orchestrator consults AllowRequest +
  AllowToolExec (block-tools lets planning run but detonates nothing). (Still
  open: expose levels in the console UI; AIDR auto-trigger.)
- **Governance inventory ✅:** `cpstore` SQLite — prompts (versioned+hashed), approvals (four-eyes decisions), mcp_pins, eval_scores, admin_audit; `LatestEval` feeds the registry promotion gate (M11→M19 MLOps loop). Fleet libs stay in-memory; course records decisions here. ✅ Wired: Governance→approvals + kill-switch→admin_audit; console `/api/history` reads it; gridge opens `inventory.db`. Open: gustoms pins→RecordPin, eval/livecheck F1→RecordEval (feeds promotion gate).
- **Email corpus ✅:** processed emails indexed into the SQLite RAG store (untrusted provenance, trace_id id) via `pipeline.Index`; `processed/` raw copy kept as source-of-truth backup (DB is rebuildable). Unlocks tracked agentic-sec topics: persistent KB/context poisoning across runs (ASI06), retrieval-as-exfil, cross-email integrity (M9 over history), DSAR/retention over the corpus. Reindex-from-disk tool tracked.
- **Persistence (decided):** SQLite (`modernc.org/sqlite`, pure-Go/no-CGO) is the embedded store wherever search or queryable history is needed — RAG ✅ done (FTS5). Next: the console's governance inventory in SQLite — prompts (versioned+hashed), config_versions+proposals+approvals (four-eyes decisions), mcp_pins, eval_scores, admin_audit. bolt dropped (no query). `memory` has a JSON snapshot (fine, no search); may fold into SQLite for uniformity. gledger stays JSONL (runtime trace ledger); inbox/outbox stays files.
- **Tracked, as budget holds:** M8 RAG, M9 conflict/confidence (overlaps B#5),
  M10 .safetensors+SCA, M12 capstone, M13 checklist, M19 MLOps/privacy, M20
  memory/C2PA, M0 threat model, Capstone II Wails wrapper, and the agentic items
  above. A complete course spans several more sessions; everything is tracked
  here so nothing is lost.

## Skeleton feature coverage (complete audit — nothing silently dropped)
✅ done · 🟡 partial · ⬜ not built
- ✅ Control-plane inventory (cpstore) · Operator RBAC (goverlord) · SoD/four-eyes · tamper-evident admin audit (cpstore admin_audit + gledger)
- ✅ Layered kill switch · versioned rollback · **IR replay** (ir/cmd/replay) · model sponge/denial-of-wallet (gouncer limits + MaxBytes) · runaway-loop breaker (gonductor MaxSteps)
- ✅ SSRF→IMDS · default-deny egress · DNS-pin/block private+link-local (netpolicy) · post-injection-exfil containment
- ✅ Model registry + promotion gates (+MLOps eval→gate loop) · output authenticity/provenance (ed25519) · memory lifecycle (TTL/scope/erasure) · MCP-as-supply-chain + pinning (gustoms) · MCP gateway · AI gateway (gouncer) · authorization-first retrieval (rag ACL-as-WHERE) · KB poisoning · Dual-LLM/CaMeL · guardrails-in-depth (gumpers) · markdown/URL exfil · trace_id logging · gorauder red-team + scorecard CI gate
- 🟡 PIR→regression (scorecard gate ✅; PIR *process* ⬜) · privacy/DSAR (memory.Erase ✅; retention/consent/legal-hold ⬜) · agent identity (a2a identity ✅, live hand-off auth ✅; delegated/attenuated token chains ⬜) · AIDR ✅ (`aidr` auto-engages kill switch on trace signals)
- ✅ **Tool-argument injection** — `argcheck` (schema + shell-metachar reject + path confinement)
- ✅ **No ambient credentials** — `captoken` (signed, scoped, short-lived per-call tokens)
- ✅ **Canary / honey-token exfil** — gumpers canary rail + ADD case
- ✅ **Clickjack-resistant approval** — `hitl` (evidence-first, nonce-echo confirm)
- ✅/N/A Degraded mode — extractor regex-fallback + gouncer fail-closed already cover it; cloud multi-region N/A (local models). IR playbooks descoped (process doc, not code; technical IR = replay+killswitch+AIDR done).
- ⬜ Dataset provenance/signing (M19). · N/A post-fine-tune regression + RLHF integrity — no training/fine-tune/RLHF pipeline in this project (pre-trained local GGUF only).
- ✅ Multi-agent collusion — `agent/quorum` (M-of-N distinct attestations; a2a spoof ✅ too) · slopsquatting / side-channels / deceptive-model-&-backdoor
- ⬜ Signed builds / SLSA / AISBOM (M17 — skipped by decision) · M0 threat model (deferred)

## Security review round 1 — findings & dispositions
- **F1 netpolicy DNS-rebinding TOCTOU (HIGH) — FIXED.** `Policy.DialContext`/`HTTPClient` resolve once and dial only the vetted IP; `WebFetchTool` uses it. Real dial-time test against a LOCAL httptest server (never public net).
- **F2 unwired shelf-ware (HIGH) — PARTLY FIXED.** Now wired into live paths: `hitl`→`cmd/controlplane` approval (evidence + nonce echo), `argcheck`→`mcp.Server` per-tool schema, `provenance`→pipeline signs each artifact (`.sig`, verified in test), `aidr`→`cmd/controlplane` scans the request trace and auto-engages the kill switch. Still provided primitives with ADD+unit tests but no *current* live consumer (honest): `captoken` (no tool needs delegated creds yet), `quorum` (single-agent flow), `dataset` (emails are unsigned→untrusted by design), `memory` (cross-run memory not enabled). Not force-wired; ready when their surface exists.
- **F3 straw-man ADD targets (MED) — ACKNOWLEDGED.** ADD cases are *control-regression gates* (prove the control rejects crafted input), not integrated end-to-end ASR. Integration is separately tested: netpolicy dial (netpolicy_test), argcheck via server (mcp_test), provenance via pipeline (pipeline_test), a2a in the live loop (a2a_loop_test), governance persistence (governance_test).
- **F4 argcheck denylist (MED) — MITIGATED.** Deployed as structured per-tool schema in the MCP server (rejects unknown args / type mismatch / metachars) + `ConfinePath`; URLs stay with netpolicy. Note retained: never build shell strings — pass arg arrays.
- **F6 captoken scope delimiter (LOW) — FIXED.** Scope is hex-encoded in the token; no separator collision.
- **F5 gustoms TOFU (LOW) — ACCEPTED, documented.** Prefer operator-supplied pins in real deployment. · a2a `seen` nonce map growth — tracked (needs TTL for long-running verifiers).

## Security review round 2 (second pass) — crypto/auth focus
- **S1 netpolicy IP blocklist gaps (LOW) — FIXED.** `blocked()` now also rejects CGNAT 100.64.0.0/10, IETF-benchmark 198.18.0.0/15, all multicast, and limited broadcast (test added). Redirect hops are re-validated (`CheckRedirect`) + dialed via the vetted-IP transport.
- **F5 gustoms TOFU — now FIXED** (not just documented): `WithStrictPinning()` refuses an unapproved server until operator `Approve` (test added).
- **a2a replay memory — FIXED:** two-generation rotation bounds the nonce set (~2×100k).
- **Reviewed clean:** a2a/captoken use `hmac.Equal` (constant-time); captoken scope hex-encoded + signed; provenance ed25519; gustoms manifest sha256 (public, timing-irrelevant); all SQL (cpstore/rag) parameterized; rag FTS5 MATCH built from `[a-z0-9]`-only quoted terms (no operator injection).
- **Accepted (defensive, rationale):** hitl/captoken use `!=` on a per-request nonce/scope — not a brute-forceable network secret (local, 96-bit), timing leak negligible. argcheck `Int` truncates JSON floats — validation clarity, not a security issue. provenance marks have no expiry — they attest authorship, not freshness (by design).

## Round 3 — product/UX build
- ✅ Reminders via `.ics` VALARM (30m before timed / 9am all-day) — native, no infra.
- ✅ M5 sanitizer hardened (reference images / raw `<img>` / autolinks / active schemes) + test.
- ✅ Capstone II = local **web app** (`webapp` + `cmd/webapp`): run the security scorecard and browse/replay incidents in-browser; reuses redteam+ir (one binary). Wails dropped.
- ✅ `docs/UX.md` — product/UX design plan (screens, principles, reminders answer, roadmap).
- AISBOM verdict: full M17 SLSA/attestation NOT needed (single-user laptop binary). A lean build manifest (models from registry + Go deps from build info + prompt hashes from cpstore) is cheap+useful — optional `cmd/sbom`, not built yet.
- Still open (UX): Drop + Events/Calendar render tab (Accept→.ics with reminder), Governance tab merge into the web app, month view.

## Round 4 — owner UX + SBOM
- ✅ Calendar tab in the web app: renders accepted `.ics` events (title/when/where, 🔔 reminder), **Accept .ics** downloads from `/ics/`. API `/api/events` parses outbox. Run: `go run ./cmd/webapp -outbox outbox`.
- ✅ `cmd/sbom` — lean AI build manifest (Go modules + model file sizes/[-hash] + prompt SHA-256).

## Round 5 — UI drop + honest sandbox status
- ✅ Drop in the UI: drag/choose/paste a .txt in the Calendar tab -> `/api/drop` writes to inbox -> watcher extracts -> events render. e2e verified.
- 🔎 MicroVM/CaMeL honesty (checked in code): MicroVM + dual-LLM are REAL but power the `cmd/controlplane` CaMeL command-execution loop, NOT the email agent (single-LLM, host-run extraction, guarded). Syllabus corrected to say so.
- ⬜ **Open gap (tracked):** route the executing MCP tool (`web_fetch`) through `Interpreter.ExecuteInSandbox`/`detonationd` so executing tools actually run in the VM. Needs `launchvm` up to test.
