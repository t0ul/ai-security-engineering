# Git Branching & Tagging Schema: AI Security Engineering

**v2 — realigned to the Email-to-Calendar Agent syllabus (`syllabus.md`, 2026-10-05).**
Supersedes the earlier 8-module scheme. Module numbers, branches, and tags now match the syllabus 1:1.

## Branch Strategy

* **`main`**: Production trunk holding verified milestone merges.
* **`module-<N>/<feature-slug>`**: Active working branch for one syllabus module.
* Tag format: **`v2.<module>-<slug>`** (v2 = curriculum version 2, the email-to-calendar pivot).

> **Migration note.** The current branch `module-3/feat-mcp-security` is misnamed for its contents — it holds Module 1 (vsock MicroVM transport), Module 3 (Presidio secret recognizer), Module 5 (exfil sanitizer) and Module 6 (HITL + circuit breaker) work, **not** MCP. Recommended: merge it to `main` as the Phase 1–2 checkpoint, then start true MCP work (Module 7) on a fresh `module-7/feat-mcp-a2a` branch. This retires the name/content drift for good.

---

## Tagging Schema

| Phase | Module | Checkpoint Tag | Working Branch | Deliverable | Status |
| --- | --- | --- | --- | --- | --- |
| 1 Foundation | M0 Threat Modeling | `v2.0-threat-model` | `module-0/threat-model` | Data-flow + trust-boundary model; CaMeL rationale | ⏳ |
| 1 Foundation | M1 Isolation & Sandbox | `v2.1-isolation` | `module-1/feat-microvm` | vz MicroVM + vsock + detonation daemon | ✅ |
| 2 Control Plane | M2 Gateway & Observability | `v2.2-gateway-obs` | `module-2/feat-gateway-observability` | LiteLLM + OTel/Langfuse tracing & audit | 🟡 |
| 2 Control Plane | M3 Privacy & Compliance | `v2.3-privacy` | `module-3/feat-pii-compliance` | Presidio + real SECRET_KEY recognizer; FERPA/GDPR framing | ✅ core |
| 3 Core Agent | M5 Output Handling & Exfil | `v2.5-output-exfil` | `module-5/feat-output-exfil` | `.ics` sanitizer; SSRF/web-search lab | 🟡 |
| 3 Core Agent | M6 Excessive Agency & HITL | `v2.6-agency-hitl` | `module-6/feat-agency-hitl` | Least-privilege tools; `.ics` accept gate; bounded breaker | 🟡 |
| 3 Core Agent | M9 Data Integrity | `v2.9-integrity` | `module-9/feat-integrity` | Deterministic date parsing; conflict detection; grounding | ⏳ |
| 4 Agentic Hardening | M4 Prompt Injection | `v2.4-injection` | `module-4/feat-injection` | Indirect injection + system-prompt leakage on real emails | 🟡 |
| 4 Agentic Hardening | M7 MCP & A2A | `v2.7-mcp-a2a` | `module-7/feat-mcp-a2a` | MCP tool scoping; A2A mutual auth; agent identity | ⏳ |
| 4 Agentic Hardening | M8 RAG & Knowledge Base | `v2.8-rag` | `module-8/feat-rag-security` | XML encapsulation; ChromaDB RBAC; memory-poisoning defense | ⏳ |
| 5 Assurance | M10 Supply Chain | `v2.10-supply-chain` | `module-10/feat-supply-chain` | safetensors/GGUF verification; dependency pinning | ⏳ |
| 5 Assurance | M11 Eval Harness & Red-Team | `v2.11-eval-harness` | `module-11/feat-eval-harness` | Ground-truth labels, prompt calibration, PyRIT + regression suite | ⏳ |
| 5 Assurance | M12 Capstone | `v2.12-capstone` | `module-12/capstone-agent` | Full-chain attack contained; forensic review | ⏳ |
| 6 Production | M13 Productionization | `v2.13-production` | `module-13/feat-production` | Reproducible provisioning; config/secrets; agent tool runtime | ⏳ |

Legend: ✅ done · 🟡 partial · ⏳ pending

> Phases 3 and 4 interleave by topic rather than strictly by module number (the core agent ships first, then hardening deepens it). Tag a module when its attack PoC + defense + eval all pass.

---

## Quick Reference Commands

```bash
# List all checkpoint tags
git tag -l -n1

# Checkout an immutable historical checkpoint without detaching HEAD
git checkout tags/<tag-name> -b review/<tag-name>

# Push a local tag to origin
git push origin <tag-name>
```
