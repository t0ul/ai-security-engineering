# Skeleton Draft — expanded scope (operator's own topic list, 2026-10)

> Captured verbatim from the author's recovered skeleton. Source for reconciling
> the syllabus toward an AI-security *platform* (governed control plane + operator GUI).

## Control plane (as a managed product)
- Inventory: prompts, tool configs, gateway policy, guardrail rules, model registry, keys

## Operator RBAC
- Deploying, edit prompt, flip policy, break glass

## Separation of Duties / four-eyes on config changes
- MCP poisoning; generalized: trust = content + approval, never the name, re-approve on change

## Tamper-evident admin-action audit
- Every control-plane change attributable; defends insider + compromised-operator threat

## AppSec of the harness
- SAST, DAST, SCA, dependency, attestation, image scanning, SIC

## Signed builds + provenance + AISBOM
- Know/verify what's in the artifact; SLSA-style attestation; sign images; verify signature at deploy

## AI-specific IR playbooks
- Named incident classes: injection, exfiltration, jailbreak, rogue loop, poisoned data
- Each with first move, roles, comms; practiced not improvised

## Layered kill switch
- Revoke agent identity/tokens -> gateway fail-closed -> scoped levels (pause sessions / block tools / full halt) -> AIDR auto-trigger -> drain + snapshot; game-day tested

## Versioned rollback of everything
- Prompts, model versions, tool defs, gateway policy as versioned artifacts; one-click rollback to known-good

## Forensics + retention
- Preserve trace_id logs + admin audit; legal hold; retention window; chain of custody; reconstructable incidents

## Provider outage / degraded mode
- Graceful degradation; fallback model or region; fail closed on safety not open

## Model-level sponge DoS / denial of wallet
- Input crafted to blow up latency/cost -> per-request token/compute/time caps

## Post-incident review -> regression
- Blameless PIR; every real incident becomes a permanent CI test so it can't recur silently

## SSRF -> cloud metadata credential theft
- Fetch/browse tool coerced to bad IP; IMDSv1 steals instance creds; IMDSv2 token + hop-limit

## Default-deny egress allowlist
## DNS pinning + block link-local / private ranges
- Stop DNS rebinding and name-based bypass; deny bad IP; RFC1918; localhost

## Tool argument injection
- Path traversal, command injection in fetch/file/shell args -> canonicalize/confine paths, arg arrays not shell strings, strict schemas

## No ambient credentials
- Nothing for a stolen request to grab; short-lived scoped per-call tokens, not env vars or instance metadata

## Combination neutralizes post-injection exfil
- Assume compromised still holds: hijacked model can't call out, reach metadata, grab creds, or pivot

## Dataset provenance + poisoning defense
- Know/sign origin of training/fine-tune/ingest data; validate at ingestion; poisoned data plants backdoors

## Post-fine-tune safety regression testing
- Fine-tuning silently degrades alignment

## RLHF annotation integrity
- Human feedback/labels are an attack surface; vet annotators; audit label quality; guard preference/reward data

## Model registry + MLOps access + promotion gates
- Who can push a model to prod; sign models; staged dev -> stage -> prod behind eval gates

## Privacy program
- Training data is personal data; purpose limitation; retention limits; consent/lawful basis; DSAR / right to erasure

## Multi-agent poisoning + collusion
- One agent's output is another's untrusted input; authenticate/validate/attenuate inter-agent (A2A) messages

## Runaway orchestration loops
- Agents calling agents, spiral in cost/actions -> depth/step/loop limits + budgets + kill switch

## Human automation bias
- People rubber-stamp confident AI; HITL that shows evidence and resists one-click; defend approval dialogs against UI-redress/clickjacking

## Output authenticity / provenance
- Mark what AI made; watermarking; C2PA content credentials (signed, tamper-evident)

## Memory lifecycle
- Persistent memory can be poisoned and hoards PII -> TTL/expiry, scrubbing, per-user/tenant scoping; recalled memory is untrusted content

## Core framings
- Agent = model + harness + tools + data + memory + integrations (that surface is yours)
- Assume the model is already compromised; non-model controls must contain it
- Lethal trifecta: private data + untrusted content + external comms
- Trust boundaries: anything the model didn't author is untrusted (tools, RAG results looping back)
- Least privilege + argument allowlists
- Deterministic authorization outside the model: model proposes, your code disposes, re-checked against real user rights
- HITL, sandboxing, spend/rate caps

## Supply chain & plumbing
- MCP as untrusted supply chain: tool poisoning, rug pulls, pin + hash, gateway
- Output handling: markdown image exfiltration, render safe, validate structured output
- Logging under one trace_id: redact, encrypt, tamper-evident, OpenTelemetry GenAI conventions
- AI Gateway: central choke point — virtual keys, budgets, guardrail hooks, egress DLP, unified logging, fail-closed
- MCP Gateway: tool-plane twin — registry allowlist of approved servers, peer tool authZ at call time, brokered scoped audience-bound tokens

## Retrieval & data
- Authorization-first retrieval: ACL inside the query
- Knowledge-base poisoning: untrusted chunks, ingestion validation, tenant isolation
- Embedding inversion

## Identity & authz
- Agent identity + delegated, attenuated tokens; per-action AuthZ

## Testing & detection
- Behavioral vs deterministic testing; where to point PyRIT; canary / honey-token exfiltration test; attack techniques
- Architectural anti-injection: Dual LLM / CaMeL; spotlighting / delimiting untrusted data
- Guardrails in depth, fail-closed: NeMo, constitutional classifiers, Bedrock automated reasoning, circuit breakers
- Runtime detection (AIDR): kill switch; feed OTEL traces in
- Side-channels; slopsquatting; deceptive model behavior + dev-time backdoor

## Governance map + CI
- OWASP LLM/ML Top 10; MITRE ATLAS; NIST AI RMF (+ GenAI Profile); ISO 42001; EU AI Act; CSA MAESTRO
- Gated CI; kill switch + replay
