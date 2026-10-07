# Pure-Go Fold-In — Project Plan & Status

## ▶ Resume here (2026-10-07) — everything green (`go test ./...`), all pushed except the Ask-School slice
Rhythm: this session runs ON the Mac (Kuebiko) — `go build`/`go test`/`git` are fine here; only booting the VM (`launchvm`, needs codesign) is a manual step. Commits authored as `t <toul@hey.com>`, ZERO Claude attribution, ONE-LINE messages. Fleet via `replace => ../fleet/*`.

**Shipped this stretch:** MicroVM egress-deny + broker + in-VM fetch (`docs/MICROVM-PLAN.md`); ADD library (`fleet/ADD`) + redteam/scorecard refactor + **fuzzing** (`docs/ADD-FRAMEWORK.md`); ensemble reconciler + promotion gate; app features A1–A7 (item types, web UI, month view — `docs/APP-FEATURES-PLAN.md`); Ask-School search (R4, this slice); **NHI capability authZ keystone complete (C4a–C4e — `controlplane.Authority`/`Grant` data-residency scoping, `webapp` bearer-grant gate, kill-switch-as-revocation ADD invariant, `rag.OpenReadOnly` least-privilege retrieval, scoped tool NHIs + `ResidencyPolicy` + gustoms MCP gateway on the app path)**. **In-app security bridges, each with a test** (`docs/INTEGRATION-GAPS.md`): PII scrub on ingest, `/ics/` leak, CSRF, year, uid, provenance signing, HITL dialog, kill switch; closed dead stubs RecordPin/RecordPrompt.

**Next (prioritized):**
- Offline keystone — **NHI identity + capability authZ (C4, `docs/CONTROL-PLANE-PLAN.md`)**: the app becomes an authZ'd API where agent/tools/MCP/RAG-reader are non-human identities holding signed, scoped, expiring capability `Grant`s (data-residency-aware — local vs frontier model decides what a token may read; `/list/contacts` is the harvest, not a read). **C4 keystone COMPLETE (C4a–C4e)** — the app is now an authZ'd API: `controlplane.Authority`/`Grant` capability tokens on the agent key; `webapp` bearer-grant gate on mutating/listing endpoints (operator token injected into the page); `Halted`←`Safety`≥`LevelHalt` kills in-flight (`RevokedTokenStillWorks` ADD invariant); `rag.OpenReadOnly` engine-enforced read-only retrieval; each tool a scoped NHI (action-fetcher = export/link, rag-reader = list/corpus, verified per call); `controlplane.ResidencyPolicy` (frontier subjects refused list/export of confidential data, swap-ready for C5); gustoms MCP on the app path via `controlplane.NewToolGateway` (pinned, allow-listed, deny-all-when-tools-blocked). 27 ADD techniques / 17 OWASP risks, all green; httptest + live smoke. **Console governance trio done** — **C6 MCP tab** (`gustoms.Gateway.Status` + pin/rug-pull/blocked + Approve) and **C2 Prompts tab** (`GovernedPrompts` over a cpstore inventory: list/version/hash/activate/reset, `RecordPrompt` now persists on the app path), both live-smoked, alongside the existing kill switch. Remaining control-plane: policies-as-governed-artifacts (C8); eval-tweak loop + Test button (C7, model-blocked); cross-tool dedup polish; Ask-School passage-chunking.

**AI-engineering reliability track (the "both hats" work — `docs/COURSE-COVERAGE.md`, blog Posts 25–28):** we exceed the Scott Moss courses on security but under-index on engineering reliability. Offline, HIGH: **durable execution + resume-verify** (checkpoint steps; resume verifies the gledger hash-chain) and **exactly-once side effects** (idempotency/action-id/version-preconditions at the tool boundary; `ResumeIntoTamperedState` + `DuplicateSideEffect` ADD cases). MED: **context compaction + summarization-injection** defense (`SummarizationInjection` ADD case). Model-blocked: pass@k/pass^k, LLM-as-judge + `JudgeManipulation`, online-eval flywheel, console Eval-trend tab (C7).
- Model-blocked (needs llama up on the Mac): LLM extractor + logical `extractor` model + grammar JSON (C5), the live ensemble, the eval-tweak loop, an LLM *answer* for Ask-School, live VM boot to verify the chamber.
- Product (Track 2): child profile (R1) → daily timeline (R3); handbook link-enrichment (R6, reuses SandboxFetchTool).

Detailed plans: `docs/{APP-FEATURES-PLAN,CONTROL-PLANE-PLAN,MICROVM-PLAN,ADD-FRAMEWORK,INTEGRATION-GAPS,COURSE-OUTLINE}.md`.

---

**Goal:** fold the six fleet libraries into the course and remove all Python,
keeping only the served-model boundary (llama.cpp, C++). Decisions (locked):
clean M0–M20 redo · local `replace → ../fleet/*` during dev · build Gonductor
first · zero Python incl. the VM daemon.

Rhythm: Claude writes Go via the device bridge; **t runs `go test` / `git` on the
Mac**. Nothing Python is deleted until the Go passes the eval (Phase 5).

## Fleet (libraries) — github.com/t0ul/*
| Repo | Role | Status |
| --- | --- | --- |
| goflage | PII/secret scrub (Presidio) | ✅ shipped |
| gledger | hash-chained audit (telemetry) | ✅ shipped |
| gouncer | LLM gateway (LiteLLM) | ✅ shipped |
| gumpers | guardrails (NeMo) + DetectEcho | ✅ shipped |
| gorauder | red-team harness (PyRIT) | ✅ shipped |
| goverlord | governed control plane | ✅ shipped + wired (controlplane.Governance: RBAC/four-eyes/rollback/kill-switch; cmd/console) |
| gonductor | orchestrator / CaMeL loop (LangGraph) | ✅ generic state-graph engine (nodes/edges/conditional routing, 2-layer circuit breaker, step hook), tested; drives the course CaMeL loop |
| gustoms | MCP gateway | ✅ shipped (M7) |
| ADD | Attack-Driven Development test library (github.com/t0ul/ADD, pkg `add`) | ✅ shipped — paired undefended/defended invariant + OWASP coverage grid over gorauder; `redteam`/`cmd/scorecard` refactored onto it |
| gridge | operator console (Wails GUI) | 🟡 HTTP backbone + served console page built in-course (controlplane.ConsoleServer + cmd/gridge, tested); Wails desktop wrapper over the same API still pending |

## Course Go port — agent/ (one module, replace → ../fleet)
| Package | Replaces (py) | Fleet used | Status |
| --- | --- | --- | --- |
| schema | schema.py | — | ✅ |
| dateparse | dateparse.py | — | ✅ (+MonthDayIndex/WeekdayIndex) |
| guard | injection_guard.py | gumpers | ✅ |
| ics | icswriter.py | — | ✅ |
| tool | tools/base.py | — | ✅ |
| extractor | tools/event_extractor.py | gumpers (via guard) | ✅ regex path tested; LLM path needs gateway |
| pipeline | process_email.py | gledger | ✅ |
| watcher | watcher.py | — | ✅ stdlib polling daemon (settle + per-cycle cap; tested regex path) |
| cmd/emaildrop | (watcher entry) | — | ✅ daemon + --once entry |
| eval | eval.py | gorauder (later) | ✅ harness + cmd/eval (pure scorer unit-tested); F1 gate run needs model |

## Phases
- **0 Scaffold** ✅ — tree + go.mod replaces.
- **1 Vertical slice** ✅ code-complete — watcher, cmd/emaildrop, eval harness all built & unit-tested on the regex path. Open item: run the F1 gate (1.00 on 3.txt, ≥0.87 on 1.txt) once llama.cpp + the gouncer gateway are up.
- **2 Gonductor + controlplane** 🟡 — gonductor engine ✅; `controlplane/` package ✅ built & tested headless (httptest gateway+MicroVM, no model): prompts, LLMClient (→gouncer), Interpreter (policy gate + MicroVM, →camel_interpreter), Middleware (→gumpers rails + goflage.Scrub), Orchestrator (gonductor graph: planner→approval→executor→coder, app+hard circuit breakers, HITL-deny-by-default, image-exfil sanitizer), cmd/controlplane demo. Remaining: run live against gouncer+llama.cpp+MicroVM; wire goverlord governance; port set-up/vm-assets detonation daemon is Phase 3.
- **3 Sandbox** 🟡 — `sandbox/` pkg + `cmd/detonationd` ✅: RunCommand (`sh -c`, 20s timeout, stderr-merge) + hand-rolled HTTP/1.1 framing (OS-independent, unit-tested on darwin via net.Pipe + TCP); AF_VSOCK listener via x/sys/unix build-tagged linux-only, darwin stub keeps host tree buildable; cross-builds GOOS=linux/arm64. Protocol matches controlplane.Interpreter ({command,trace_id}→{output,trace_id}). DAEMON_TCP env for host dev. Remaining: build into the Alpine initramfs (replace the .py in vm-assets), live vsock test; launch_vm.go stays Go already.
- **4 Red-team** 🟡 — `redteam/` pkg ✅: 5 technique cases (indirect-injection→guard.Sanitize, prompt-leak→guard.DetectPromptLeak, url-exfil→controlplane.SanitizeMarkdown [now exported], pii→goflage.Scrub, sponge→pipeline.MaxBytes), each run as gorauder seeds vs undefended+defended targets scored by BlockAwareScorer; tests assert ASR 1.00→0.00 per technique + overall. `AllSeeds()` ready to harvest. Remaining: harvest generic seeds into the gorauder lib itself; wire ASR into eval security-rate output; add live-LLM target + converters (filter-evasion).
- **5 Rip out Python** ✅ — all `.py` deleted (module-2/, module-3/, set-up/vm-assets/{detonation_daemon,agent_harness}.py + requirements.txt), three venvs removed, __pycache__ swept. Eval data (samples + labels) migrated to `testdata/emaildrop/`; `cmd/eval` repointed. `remaining .py: 0`; agent/controlplane/sandbox/redteam suites all green.
- **6 Docs** ✅ — scratch/syllabus.md rewritten to v3: "stays Python" decision reversed (now zero-Python, one Go module + fleet, served model only); architecture diagram + trace stack + hardware blueprint + modules M1–M13 + decision section + roadmap + ADD ledger all redrawn onto goflage/gumpers/gouncer/gonductor/gledger/gorauder; ledger reframed to gorauder ASR cases (100%→0%). Grep-verified: no stale Presidio/NeMo/LiteLLM/LangGraph/watcher.py/detonation_daemon.py references except the fleet "Replaces" column + why-reverse prose.

## Python still present
None — repo is zero-Python as of 2026-10-06 (Phase 5 complete). Only non-Go runtime is the served model (llama.cpp, C++) behind gouncer. Orphan left on purpose: `set-up/vm-assets/agent_harness.sh` (shell, drove the deleted harness) — delete if unwanted.

## Definition of done
- ✅ Zero `.py` in the repo.
- ✅ `go test ./...` green **repo-wide** — dup-`main` resolved (set-up mains → cmd/ sharing internal/assets; scratch snapshots build-tagged); every package ok.
- ✅ eval F1 ≥ Python — **live run cleared both gates**: 3.txt F1=1.00, 1.txt F1=1.00 (13/13, P=R=1.00), well past the ≥0.87 target. Root-caused a regression where the output-side prompt-leak guard false-positived on the prompt's own few-shot example (JSON-shaped) and dropped all 1.txt candidates; fixed by scoping the leak reference to the instructions only (`extractionLeakRef`, excludes the Examples block). `cmd/livecheck` runs the whole gate with one command (starts models ready-gated, gouncer in-process, scores both labels); it auto-reclaims the model/gateway ports (`modelserve.ReclaimPort`) so it is re-runnable without manual pkill.
- ✅ redteam ASR drops to ~0 behind controls — 8/8 → 0/8 across 5 techniques.
- ✅ syllabus rewritten (v3).
- ⬜ fleet importable by version tags (swap `replace => ../fleet/*` at the very end).

## Remaining (post-fold-in)
- dup-`main` cleanup 🟡: `scratch/*-worked.go` build-tagged `//go:build ignore`; `set-up/` mains refactored into `cmd/launchvm`, `cmd/prepareassets`, `cmd/verifymodel` sharing new `internal/assets` (one verified-download path, tested). **Awaiting `git rm` of the 3 old `set-up/*.go` + stale `set-up/launch_vm` binary** (harness blocked the delete) to make `go build ./...` green repo-wide.
- LLM automation ✅: `internal/modelserve` supervisor + `cmd/modeld` launch both llama.cpp servers (planner :11435, coder :11436), health-gate, prefixed logs, clean shutdown. **Verified live on the Mac — both models load & serve from one command.** Hardened after a double-run exposed a false-ready: now refuses to start over an already-serving port and aborts if a child exits during startup. Readiness is a pluggable probe — `modeld` gates on a real 1-token completion (llama returns 503 while loading), so "ready" means the model actually generated, not just that the socket is open. All tested. Replaces the two hand-run `llama serve` shells.
- Live-run wiring ✅: `docs/RUNBOOK.md` (ordered 3-terminal bring-up + live checks), `set-up/gouncer.json` (routes planner→11435, coder→11436, :4000), `cmd/launchvm` doc documents the required `codesign --entitlements set-up/entitlements.plist -s - --force` step (vz needs the virtualization entitlement; `go run` fails). Extractor sends model `"planner"`; GATEWAY_URL defaults to `:4000`.
- VM path de-pythoned ✅: `sandbox_init.sh` rewritten (no pip/python) to launch the static Go `detonationd`; `cmd/launchvm` bring-up drops `apk add python3`; `cmd/prepareassets` now cross-compiles `detonationd` (linux/arm64, CGO off — verified static ELF) into `vm-assets/`. Full chain Go: prepareassets→launchvm→sandbox_init→detonationd(vsock:5000)→bridge→controlplane.Interpreter. Live boot test pending (needs the Mac + a real VM run).
- F1 gate ✅ cleared live (both labels 1.00 via `cmd/livecheck`). Still unexercised live: the controlplane CaMeL demo + detonationd over the real vsock bridge (needs `launchvm` + the codesign step; eval path does not).
- Harvest generic redteam seeds into gorauder; add live-LLM target + converters.
- Goverlord governance ✅ wired (Track II M14/M15): `controlplane.Governance` puts the agent's operational config under RBAC + four-eyes approval + versioned rollback + fail-closed kill switch, audited to gledger; `Orchestrator.Killed` hook halts the loop when the switch is engaged; `cmd/console` demos the full lifecycle (chain verifies). Operator console backbone ✅: `controlplane.ConsoleServer` serves the governed plane as a JSON API + a live wired-to-state HTML console (`cmd/gridge`), goverlord enforced server-side, errors mapped to HTTP status (403/409/422/…), tested with httptest. Remaining Track II: the Wails desktop wrapper (fleet gridge repo) over this same API — Capstone II.
- Swap local `replace` directives → version tags for release.

## Next: control-plane hardening — see `docs/CONTROL-PLANE-PLAN.md`
Make the control plane a governed-artifact registry. Headline gap: **prompts are
hardcoded consts** (`PlannerSystemPrompt`/`CoderSystemPrompt`/`ExtractionPrompt`)
— `cpstore.RecordPrompt` can store versioned/hashed prompts but nothing resolves
them at runtime, so there's no prompts management plane. Plan: prompt resolver +
pin + governed lifecycle (propose→approve→gate→activate→rollback), a logical
`extractor` model for config-driven model swaps, the extraction ensemble +
ExtractionInjection/HallucinationReconcile ADD cases, and an eval-F1 + ADD-ASR
promotion gate on every prompt/model change. Phases C1–C6 (t implements).

## Next: app features beyond meetings — see `docs/APP-FEATURES-PLAN.md`
Owner-use app upgrade: non-meeting **tasks/reminders** (buy popcorn, clothes
donation, permission slips), **heads-ups** (guest author visiting), **actions**
(accept classroom-app invite / RSVP links, netpolicy+VM-brokered), plus
surfacing the already-built read-only tools (digest, contacts, action-items) and
a needs-review queue + month view. Item-kind model on `schema.Event`, `VTODO` in
`ics`, pipeline summary sidecars, new web endpoints + tabs. Phased A1–A8.

## MicroVM hardening ✅ (host-side) — see `docs/MICROVM-PLAN.md`
Made the MicroVM a real egress-denied, ephemeral detonation chamber and exposed
it to the agent as the governed `sandbox_exec` MCP tool. All six slices landed &
tested on the Mac: P1 egress-deny (no network device, no `udhcpc`/`apk`/NTP) ·
P4 allowlist (`controlplane.Policy` argv allowlist + argcheck arg-arrays; blocklist
demoted to tripwire; ADD `AllowlistBypass` 100%→0%) · P2 `controlplane.SandboxExecTool`
routing argv through the VM, wired via gustoms in `cmd/mcpdemo` · P3 ephemeral
per-detonation temp cwd · P5 non-root guest user · P6 docs. web_fetch stays
host-side behind netpolicy on purpose (the chamber has no egress). **Only t-step
left:** re-boot the VM (`launchvm` + codesign) to confirm the no-egress/non-root
acceptance checks live.
