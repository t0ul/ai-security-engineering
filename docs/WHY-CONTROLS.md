# Why each control exists — the Attack-Driven Development ledger

**The rule of this project: we never add a security control without first proving
the attack it stops.** Every control ships as a *pair* of tests — the attack
landing **without** the control, and the same attack **failing with** it:

- **Undefended target → ASR 100%.** The exploit succeeds, so the threat is real,
  not theoretical. If an "undefended" case can't reach 100%, the attack isn't
  demonstrated and the control isn't justified yet.
- **Defended target → ASR 0%.** The real control (from the code the app runs)
  kills the same attack.

That before→after *is the justification*. A reader sees the exploit work, then
sees our defense stop it, so every line of security code points back to a
demonstrated threat. This is **Attack-Driven Development (ADD)**; the paired
invariant is something a plain unit test can't express.

## See it yourself
- **Offline (no model, deterministic, CI):** `go run ./cmd/scorecard` prints the
  whole ledger — `technique | risk | before 100% | after 0% | ok` for every case.
  `go test ./pkg/redteam/` enforces the invariant (undefended 100%, defended 0%).
  These are white-box in-process pairs: they prove the **controls**, fast, with no
  llama/qwen (see `docs/ADD-FRAMEWORK.md`).
- **Live (real llama/qwen):** `go test -tags live ./pkg/livetest/ -v` runs the
  model-dependent attacks through the actual models — `TestLiveScorecard` (5 cases)
  and `TestLiveEvalF1` / `TestLivePassK`. Honest rule live: gate on the defended
  path, *observe* undefended (a live 3B may resist a payload on its own — reported,
  never faked).

## The ledger (headline pairs; `cmd/scorecard` is the full, living list)
Each row: the control, the attack it answers, and the before→after evidence.

| Control (what we built) | Attack it stops (the why) | Evidence |
| --- | --- | --- |
| `agent/guard` + CaMeL quarantine + gumpers | Indirect prompt injection in an email rewrites the agent's behavior | `indirect-injection` / `extraction-injection` 100%→0% (+ live) |
| `rag.Assemble` encapsulation + governed `chat_system` prompt | Poisoned corpus doc hijacks the chat answer; the chat trusts an email's date | `chat-rag-injection` 100%→0%; prompt-isolated chat-injection via the Prompts **Test** button (live); date comes only from the host clock, never the corpus |
| `guard.DetectPromptLeak` | Model coerced to echo its own system prompt | `prompt-leak` 100%→0% (flips **live**) |
| `ics.SanitizeField` / `SanitizeMarkdown` | `.ics`/output exfil via tracking pixels, `javascript:`/`data:` URLs | `url-exfil` 100%→0% |
| `goflage` PII scrub | Staff/student PII or secrets echoed out of the agent | `pii-secret-leak` / `canary-exfil` 100%→0% |
| `netpolicy` default-deny egress + DNS pin | SSRF to cloud metadata (169.254.169.254), link-local/RFC1918 | `ssrf-imds` / `action-link-exfil` 100%→0% |
| `argcheck` + argv allowlist | Shell/command injection through a tool argument | `arg-injection` / `allowlist-bypass` 100%→0% |
| Apple `vz` MicroVM (egress-deny, non-root, broker) | LLM-suggested command runs on the host | the chamber + `AllowlistBypass`; live VM smoke |
| `gustoms` manifest pinning | Rug-pull: an MCP server silently swaps its tools | `mcp-rug-pull` 100%→0% |
| `a2a` signatures + `quorum` | Forged planner→executor message; one rogue agent acting alone | `a2a-spoof` / `multi-agent-collusion` 100%→0% |
| `controlplane.Authority` capability grants | Stolen/over-scoped token; frontier identity harvesting data | `revoked-token-still-works` + identity tests; residency `IssuePolicy` |
| `controlplane.Safety` kill switch (+ `Authority.Halted`) | Acting after containment is engaged; token still honored | `killswitch-bypass` / `revoked-token-still-works` 100%→0% |
| `rag.Assemble` encapsulation + tenant ACL | KB poisoning; cross-tenant retrieval | `rag-poisoning` / `rag-tenant-leak` 100%→0% |
| `dateparse` + `ensemble.Reconcile` | Hallucinated date landed on the calendar | `hallucination-reconcile` 100%→0% (flips **live**) |
| `assets.CheckModelFormat` | Code-executing pickle model artifact | `model-supply-chain` 100%→0% |
| `registry` promotion gate (eval F1 + ADD ASR) | Unsigned/ungated model shipped to prod | `promotion-gate-bypass` 100%→0% |
| `hitl` evidence+nonce confirm | Clickjacked / forged one-click approval | `approval-forgery` 100%→0% |
| `provenance` content credentials | Tampered output passed off as the agent's | `output-forgery` 100%→0% |
| `durable` resume-verify | Agent resumed into a tampered checkpoint | `resume-tampered-state` 100%→0% |
| `durable` idempotency key | Retried/duplicated action fires a side effect twice | `duplicate-side-effect` 100%→0% |
| `compaction` provenance partition | Injection laundered into a trusted running summary | `summarization-injection` 100%→0% |
| encapsulated LLM-as-judge | Eval gamed by prompt-injecting the judge | `judge-manipulation` 100%→0% |
| `aidr` auto-contain | A fired detection goes unactioned | `aidr-uncontained` 100%→0% |

31 techniques, 0 regressed. When a new control is proposed, the first commit is
its attack (an `add.Technique` whose undefended target hits 100%); the control
lands in the same or next commit and drives it to 0%. No attack, no control.
