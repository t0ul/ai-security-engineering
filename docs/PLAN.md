# Pure-Go Fold-In — Project Plan & Status

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
| goverlord | governed control plane | ✅ shipped |
| gonductor | orchestrator / CaMeL loop (LangGraph) | ✅ generic state-graph engine (nodes/edges/conditional routing, 2-layer circuit breaker, step hook), tested; drives the course CaMeL loop |
| gustoms | MCP gateway | ⬜ later (M7) |
| gridge | operator console (Wails GUI) | ⬜ last (Capstone II) |

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
- 🟡 eval F1 ≥ Python — regex path 3.txt=1.00; LLM path 1.txt ≥0.87 gate needs llama.cpp + gouncer up (live run pending).
- ✅ redteam ASR drops to ~0 behind controls — 8/8 → 0/8 across 5 techniques.
- ✅ syllabus rewritten (v3).
- ⬜ fleet importable by version tags (swap `replace => ../fleet/*` at the very end).

## Remaining (post-fold-in)
- dup-`main` cleanup 🟡: `scratch/*-worked.go` build-tagged `//go:build ignore`; `set-up/` mains refactored into `cmd/launchvm`, `cmd/prepareassets`, `cmd/verifymodel` sharing new `internal/assets` (one verified-download path, tested). **Awaiting `git rm` of the 3 old `set-up/*.go` + stale `set-up/launch_vm` binary** (harness blocked the delete) to make `go build ./...` green repo-wide.
- LLM automation ✅: `internal/modelserve` supervisor + `cmd/modeld` launch both llama.cpp servers (planner :11435, coder :11436), health-gate, prefixed logs, clean shutdown. **Verified live on the Mac — both models load & serve from one command.** Hardened after a double-run exposed a false-ready: now refuses to start over an already-serving port and aborts if a child exits during startup. Readiness is a pluggable probe — `modeld` gates on a real 1-token completion (llama returns 503 while loading), so "ready" means the model actually generated, not just that the socket is open. All tested. Replaces the two hand-run `llama serve` shells.
- Live-run wiring ✅: `docs/RUNBOOK.md` (ordered 3-terminal bring-up + live checks), `set-up/gouncer.json` (routes planner→11435, coder→11436, :4000), `cmd/launchvm` doc documents the required `codesign --entitlements set-up/entitlements.plist -s - --force` step (vz needs the virtualization entitlement; `go run` fails). Extractor sends model `"planner"`; GATEWAY_URL defaults to `:4000`.
- VM path de-pythoned ✅: `sandbox_init.sh` rewritten (no pip/python) to launch the static Go `detonationd`; `cmd/launchvm` bring-up drops `apk add python3`; `cmd/prepareassets` now cross-compiles `detonationd` (linux/arm64, CGO off — verified static ELF) into `vm-assets/`. Full chain Go: prepareassets→launchvm→sandbox_init→detonationd(vsock:5000)→bridge→controlplane.Interpreter. Live boot test pending (needs the Mac + a real VM run).
- One live end-to-end run (gouncer + llama.cpp + MicroVM): close the 1.txt F1 gate, exercise controlplane + detonationd over real vsock.
- Harvest generic redteam seeds into gorauder; add live-LLM target + converters.
- Wire goverlord governance into the control plane (Track II).
- Swap local `replace` directives → version tags for release.
