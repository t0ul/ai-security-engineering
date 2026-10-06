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
| M5 | output handling / exfil | 🟡 markdown-image strip + SSRF/web-fetch lab (mcp web_fetch + netpolicy) ✅; hardened sanitizer (`<img>`, reference/autolinks, `js:`/`data:`) still missing |
| M6 | excessive agency / HITL / circuit breaker | ✅ |
| M7 | MCP & A2A security (gustoms) | ✅ gustoms gateway + mcp transport + agent/a2a (wiring a2a into live loop pending) |
| M8 | RAG / KB security | ✅ `rag` on SQLite FTS5 (pure-Go, no CGO): real query, tenant-ACL-as-WHERE, provenance, sanitized recall; poisoning & tenant-leak ADD cases. Vector/semantic (embeddings via gouncer) + embedding-inversion tracked |
| M9 | data integrity / misinformation | 🟡 weekday-integrity dateparse; cross-source conflict + confidence scoring missing |
| M10 | supply chain / model artifacts | ✅ `assets.CheckModelFormat` (reject pickle) + gguf magic/sha + safetensors header verify; dep-SCA tracked |
| M11 | eval harness + red-team automation (gorauder ASR) | ✅ |
| M12 | full-chain capstone | ✅ `cmd/scorecard` (15 techniques ASR, CI gate) + `ir`/`cmd/replay` (reconstruct an incident by trace_id from the chain-verified log) |
| M13 | productionization / deployment | 🟡 runbook + static binaries; checklist + health/degraded modes missing |

## Track II — operate & govern
| Module | Capability | Status |
| --- | --- | --- |
| M14 | governed control plane + operator RBAC | ✅ |
| M15 | admin audit + versioned rollback | ✅ |
| M16 | network/credential containment (egress allowlist, DNS pinning, block link-local/RFC1918, SSRF→IMDS) | ✅ `netpolicy` (arg-schema hardening + no-ambient-creds still open) |
| M17 | AppSec/supply-chain of the harness (SAST/DAST/SCA, SLSA, AISBOM, signed images) | ⬜ **SKIPPED (operator's call)** |
| M18 | IR playbooks + layered kill switch + **IR replay** + AIDR + degraded + PIR→CI | 🟡 kill switch ✅, IR replay ✅ (`ir`/`cmd/replay`); playbooks + AIDR auto-trigger + degraded mode + PIR→CI pending |
| M19 | MLOps/data/privacy | 🟡 `registry` promotion gates + DSAR erasure (`memory.Erase`); dataset provenance / post-fine-tune / RLHF tracked |
| M20 | memory lifecycle + output authenticity + MCP gateway | ✅ `memory` (TTL/scope/erasure/untrusted-recall) + `provenance` (ed25519 content credentials) + MCP gateway (gustoms) |
| Capstone II | operator console GUI | 🟡 HTTP backbone + served page (cmd/gridge); Wails wrapper pending |

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
- **C — A2A security (M7/ASI07).** ✅ DONE: `agent/a2a` — HMAC-signed inter-agent
  messages, per-agent keys, replay guard; forged/tampered/unknown/replay all
  rejected. (Still open: wire it into the live gonductor hand-offs.)
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
