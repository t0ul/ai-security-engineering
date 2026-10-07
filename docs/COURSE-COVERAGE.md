# Coverage audit — Scott Moss AI-engineering courses vs our AI-security course

`docs/course.txt` is four concatenated Master.dev / Frontend Masters TOCs by Scott
Moss (the repos: `Hendrixer/ai-engineering-fundamentals` = the Excalidraw agent +
evals + context + RAG; `harness-engineering` = durable execution, sandbox/code
mode, memory/compaction, supervision; `agents-v2` = AI Agents Fundamentals v2,
the agent loop + evals + FS/web/shell tools + approvals; `background-agents` =
long-running autonomous agents, idempotency, exactly-once side effects).

Those are **AI-engineering** courses (build + measure + improve an agent). Ours is
**AI-security** engineering. This is an honest two-way check: (1) do we build/teach
each engineering topic, and (2) do we add the security control it needs. Status:
✅ match-or-exceed · 🟡 partial · ❌ gap. Code-verified 2026-10-07.

## Where we EXCEED (security the courses don't teach)
- **Sandbox / code execution.** They: Docker/Daytona + timeout. Us: Apple `vz`
  MicroVM — egress-deny + broker, non-root, ephemeral, argv allowlist + argcheck,
  vsock-only, MCP-gated. ✅✅
- **Tool least privilege** ("treat the LLM as a user who sees only the minimum").
  Us: goflage PII scrub on ingest + capability scoping (C4) + data-residency
  `IssuePolicy` (frontier identity can't `list`/`export` confidential). ✅✅
- **RAG security.** poisoning (rag.Assemble encapsulation + sanitize), tenant-leak
  (authorization-first WHERE), provenance tags, read-only handle (C4d), PII scrub.
  The course treats RAG as pure capability. ✅✅
- **Supervision / sub-agents.** Their read-only investigators = our quarantined
  Q-LLM + capability-scoped read-only NHIs + a2a-signed handoff + quorum. ✅✅
- **HITL.** Their approval gate = our clickjack-resistant evidence+nonce confirm
  (ApprovalForgery ADD case). ✅
- **Attack-Driven Development.** 27 techniques / 17 OWASP risks, ASR 100→0. The
  courses have **no** security evals at all. ✅✅
- **Audit tamper-evidence.** gledger hash-chain + ir replay. Their durable event
  log is not tamper-evident. ✅

## Solid MATCH
| Topic (course) | Us | Security control |
| --- | --- | --- |
| Agent loop / harness | gonductor state-graph + CaMeL | circuit breakers, HITL-deny-by-default |
| Tool calling + schema validation | mcp.Tool + argcheck + schema | arg-array not shell-string, schema reject |
| Web search tool | SandboxFetchTool (VM) | netpolicy SSRF allowlist, ActionLinkExfil |
| Golden dataset + code-based scorer | eval harness (F1 P/R on labels) | — |
| Regression evals | ADD + F1 gate + promotion gate | eval-F1 + ADD-ASR fail-closed |
| Context eng: prompt rewrite, few-shot, guardrails | prompts plane (C1/C2) + gumpers | governed/versioned/hashed prompts, injection defense |
| Observability / tracing | gledger trace_id across hops + ir | value-redaction in logs (no PII) |
| Deterministic workflows | dateparse, regex extractor, trusted control-flow | determinism-first |
| Memory | memory pkg (recall, TTL/scope) | memory-poisoning neutralize |
| Shell / code mode | sandbox_exec (argv) in MicroVM | allowlist + argcheck + isolation |

## PARTIAL
- **Structured outputs / JSON-schema-constrained gen (GBNF).** Planned C5, model-blocked. We validate output but don't constrain generation yet.
- **Sampling governance (temperature/seed).** `CompleteOpts` literals exist; not governed. Planned C5.
- **Online evals / data flywheel.** Human accept/review + audit exist; no closed loop feeding evals. C7 planned, model-blocked.
- **Async / durable HITL (pause for days).** Ours is synchronous nonce confirm.
- **Parallel sub-agent dispatch.** We route + handoff; no concurrent fan-out+synthesis.
- **FS tools (read/write/list/delete).** Writes are scoped (.ics outbox, serveICS .ics-only); no generic FS tool, but the `ActionList`-is-harvest control pattern is there.

## GAPS — engineering topic we do NOT build (so no control hangs on it)
| # | Gap | Why it matters (incl. the missing security control) | Priority |
| --- | --- | --- | --- |
| 1 | ✅ **DONE — Durable execution + resume-verify** (`durable` pkg: hash-chained step log, `Do` replays cached steps across reopen) | Resume replays completed steps; `Open`/`Verify` fails closed on a tampered checkpoint (`ResumeIntoTamperedState` ADD case, ASI10). Wired in cmd/webapp (action fetch; tamper on boot → start fresh + audit). | ~~HIGH~~ done |
| 2 | ✅ **DONE — Exactly-once side effects** (`durable.Do` idempotency key: first success cached, replay returns it, failure retryable) | A duplicated/replayed action never re-fires the effect (`DuplicateSideEffect` ADD case, ASI08). Wired on the action-fetch tool boundary; extends the single-use HITL nonce. Version-precondition/ack-gap can layer on later. | ~~HIGH~~ done |
| 3 | ✅ **DONE — Context compaction + summarization-injection defense** (`compaction` pkg: running-summary + sliding-window under a token budget) | Provenance-partitioned: trusted/untrusted turns summarized separately, the untrusted summary stays `Trusted=false` + guard-sanitized, so an injected instruction can't be laundered into trusted context (`SummarizationInjection` ADD case, LLM01). Package + ADD shipped; integration point is the multi-turn orchestrator's context assembly (the single-shot email webapp has nothing to compact — not fake-wired). | ~~MED~~ done |
| 4 | **LLM-as-judge / model-graded evals** (judge schema, score+reasoning, multi-turn judging) | We're deterministic-only. Missing control: **judge manipulation / eval-gaming** (prompt-inject the judge to pass). | MED (model-blocked) |
| 5 | **pass@k / pass^k reliability evals** | We score once, deterministically. No quantification of non-deterministic reliability over k samples. | MED |
| 6 | **Observability dashboard / OpenTelemetry + eval-over-time trend** | gledger+ir+scorecard exist; no metrics trend / OTel export / console Eval tab. C7 (console Eval tab) planned. | MED |
| 7 | **RAG query rewriting / query-quality step** | Not built. Control note: an LLM query-rewriter is itself an injection surface. | LOW |
| 8 | **Tool-selection precision eval** (did the planner pick the right tool?) | We eval extraction F1, not planner tool choice. | LOW |
| 9 | **Token-efficient context serialization (TOON vs JSON)** | Minor; we pass plain text. A token-budget discipline topic. | LOW |
| 10 | **Streaming output + incremental output handling** | Our product is batch; streaming has its own output-handling/XSS surface we don't exercise. | LOW |

## Deliberate non-goals (not gaps)
- Managed agent frameworks (OpenAI Agents SDK, Mastra, Voltagent, Browserbase,
  Director), Cloudflare Durable Objects, Braintrust/Laminar SaaS, client-side
  browser tools, Upstash/Neon managed infra, TypeScript/React stack — different
  (Go, local, zero-Python, self-hosted) by design.
- "Code mode" = arbitrary LLM-generated code: we deliberately run an argv
  allowlist, not arbitrary code. We have the isolation to demo true code-mode
  safely if we ever want it (a bounded post).

## Verdict
On **security depth** we exceed every course (injection, exfil, SSRF, sandbox,
identity/authZ, MCP, RAG, provenance, HITL, kill switch, ADD). The three offline
reliability holes are now **closed**: durable execution + resume-verify (#1) and
exactly-once side effects (#2) in the `durable` package, and context compaction +
summarization-injection defense (#3) in the `compaction` package — each with an
ADD invariant (`ResumeIntoTamperedState`, `DuplicateSideEffect`,
`SummarizationInjection`). Remaining is the eval methodology (#4–#6: pass@k,
LLM-as-judge + judge-manipulation, online-eval flywheel, eval-trend tab). These
were called "model-blocked" but the live path is now proven (livecheck + livetest,
F1=1.00/1.00; a live ADD scorecard runs in `livetest/`), so they are **live-doable**
— they need `modeld` serving, not anything we lack. Plan in `docs/CONTROL-PLANE-PLAN.md`
(C5/C7) and blog Posts 28–29.
