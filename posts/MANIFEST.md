# Manifest

The single source of truth for the book's order and each lesson's brief.
`make status` reads this and the filesystem; `make new POST=<id>-<slug>` copies a
row into the new folder's `SPEC.md`. Ids are zero-padded (`03.1`, `10.2`) so
folders sort in book order. Keep specs on one line and never use `|` inside one.

| id | slug | part | title | spec |
|----|------|------|-------|------|
| 00.0 | introduction | P0 front matter | Start here: how to use this handbook | prose only -> no attack -> intro + repo orientation |
| 00.1 | the-plan | P0 front matter | The plan: a local email-to-calendar agent, built to be attacked | prose only -> the product, why security, the loop, full roadmap |
| 00.2 | setting-up-the-lab | P0 front matter | Set up your lab: from a bare Mac to a running agent | prose only -> brew go + llama.cpp, clone emaildrop, build, regex run, eval f1=1.00, modeld |
| 00.3 | attack-driven-design-shown-once | P0 front matter | Attack-Driven Design, shown once | one loop turn -> model invents Dec 25 gala not in email -> ensemble.Reconcile drops off-text dates -> receipt: hallucination 100% to 0% |
| 01.1 | one-shape-for-everything | P1 ch1 parser | One shape for everything | schema.Event typed contract -> no attack (foundation) -> show the one struct every stage agrees on |
| 01.2 | dates-the-model-never-decides | P1 ch1 parser | Dates the model never decides | dateparse -> stated weekday mismatch + Feb 30 -> naive parse calendars a wrong date -> weekday integrity + reject impossible -> flagged not calendared |
| 01.3 | inert-output-and-the-human-accept | P1 ch1 parser | Inert output and the human accept | ics writes a file -> auto-add would push a bad extraction onto a real calendar -> inert .ics + accept gate |
| 01.4 | the-watched-folder | P1 ch1 parser | The watched folder | watcher -> oversized file + half-written read -> size cap + settle delay + per-cycle cap |
| 02.1 | the-eval-harness | P1 ch2 prove it | The eval harness: F1 as a receipt | eval over labeled samples -> no attack -> f1=1.00 on clean mail is the baseline receipt |
| 02.2 | attacking-the-baseline | P1 ch2 prove it | Attacking the baseline, all red | whole undefended agent -> run every attack -> injection, leak, PII, exfil all land at 100% -> this red board is the TOC |
| 03.1 | a-log-you-can-trust | P2 ch3 observability | A log you can trust | gledger -> forge a plain log + the 0-events blindness -> hash chain + redact + defang -> chain_ok=false, trace explains |
| 03.2 | a-gateway-you-cannot-route-around | P2 ch3 observability | A gateway you cannot route around | gouncer -> SSRF to any host + flood -> model allowlist + body, rate, concurrency caps -> 400 and 429 |
| 03.3 | one-trace-id | P2 ch3 observability | One trace id, and the command stream | correlation id across stages -> no attack -> the operator window for free |
| 04.1 | when-the-email-gives-orders | P3 ch4 injection | When the email gives orders | indirect injection found by accident -> 4 seeds land -> guard.Sanitize + gumpers -> 100% to 0% |
| 04.2 | the-email-is-data-not-orders | P3 ch4 injection | The email is data, not orders | spotlighting / instruction hierarchy prompt layer -> sender text outranks mine -> delimited data channel -> blocked |
| 04.3 | system-prompt-leakage | P3 ch4 injection | System-prompt leakage | prompt-leak seed coaxes instructions out -> DetectPromptLeak on output -> LLM07 0% |
| 05.1 | scrubbing-secrets-and-pii | P3 ch5 secrets and output | Scrubbing secrets and PII at ingest | goflage -> pii-secret-leak: tokens and PII reach disk -> scrub at ingest -> none in corpus |
| 05.2 | the-calendar-field-exfil | P3 ch5 secrets and output | The calendar-field exfil | url-exfil via URL and NOTES fields -> field sanitizer -> LLM05 0% |
| 05.3 | the-tracking-pixel | P3 ch5 secrets and output | The tracking pixel | markdown image and link exfil on render -> output allow-listing -> blocked |
| 05.4 | a-search-tool-hits-a-local-server | P3 ch5 secrets and output | A web-search tool that hits a local server | SSRF teaser -> tool reaches 127.0.0.1 -> host policy -> sets up Part 4 |
| 06.1 | least-privilege | P3 ch6 agency | Least privilege | per-tool capability scoping -> read-only tool tries to write and reach the net -> physically cannot |
| 06.2 | the-human-gate | P3 ch6 agency | The human gate | HITL deny by default -> approval-forgery: persuasive text drives a one-click Accept -> gate holds |
| 06.3 | circuit-breakers | P3 ch6 agency | Circuit breakers and runaway loops | gonductor cap -> sponge-dos runaway loop -> breaker trips -> LLM10 0% |
| 07.1 | the-problem-with-running-code | P4 ch7 isolation | The problem | naive agent runs a model-suggested shell command -> owns the Mac -> motivates the chamber |
| 07.2 | the-detonation-chamber | P4 ch7 isolation | The detonation chamber | Apple VM, vsock only, no NIC -> code runs with no path to host -> live VM smoke |
| 07.3 | egress-deny-and-the-broker | P4 ch7 isolation | Egress-deny and the one way out | egress denied by default -> allowlist broker is the one audited exit |
| 07.4 | no-shell | P4 ch7 isolation | No shell | argv arrays + argcheck allowlist -> allowlist-bypass + arg-injection -> 0% |
| 08.1 | the-dual-llm-brain | P4 ch8 dual-LLM | The dual-LLM brain (CaMeL) | privileged planner decides, quarantined reader touches untrusted text but cannot act -> model output is data |
| 08.2 | agent-to-agent-hand-off | P4 ch8 dual-LLM | Agent-to-agent hand-off | authenticate planner to executor -> a2a-spoof -> 0% |
| 09.1 | pin-the-tools | P4 ch9 protocol and supply chain | A pinned tool catalog (MCP) | gustoms -> mcp-rug-pull swaps the catalog -> pin -> 0% |
| 09.2 | model-artifacts | P4 ch9 protocol and supply chain | Model artifacts | malicious .pkl vs verified .gguf -> magic + hash check -> pickle 0% |
| 09.3 | defeating-the-lethal-trifecta | P4 ch9 protocol and supply chain | Defeating the lethal trifecta | netpolicy + captoken + argcheck together -> ssrf-imds -> 0% |
| 10.1 | answering-whats-on-this-month | P5 ch10 RAG and memory | Answering what is on this month | build retrieval (GoRag) -> no attack -> the thing Part 5 then attacks |
| 10.2 | index-poisoning | P5 ch10 RAG and memory | Index poisoning | rag-poisoning planted doc steers answers -> XML-encapsulated chunks -> 0% |
| 10.3 | cross-tenant-leak | P5 ch10 RAG and memory | Cross-tenant leak | rag-tenant-leak -> authz-first retrieval, ACL in the query -> 0% |
| 10.4 | memory-poisoning | P5 ch10 RAG and memory | Memory poisoning and lifecycle | memory-poisoning -> TTL, scope, sanitized recall -> 0% |
| 11.1 | conflicts-and-mangled-data | P5 ch11 integrity | Conflicts and mangled data | Tue vs Thu, borrowed time, negation inversion, dup collapse -> deterministic checks -> caught |
| 11.2 | hallucinated-dates | P5 ch11 integrity | Hallucinated dates | ensemble.Reconcile drops off-text dates -> hallucination-reconcile -> 0% (full treatment of 00.3) |
| 12.1 | the-one-command-scorecard | P6 ch12 flywheel | The one-command scorecard, all green | run the whole board -> every row 100% to 0% -> reproducible in one command |
| 12.2 | from-pattern-to-library | P6 ch12 flywheel | From pattern to library | extract the loop into the ADD go-test library -> no attack -> the method as code |
| 12.3 | eval-methodology | P6 ch12 flywheel | Honest eval methodology | pass@k and model-as-judge -> judge-manipulation -> hardened judge |
| 12.4 | live-red-team | P6 ch12 flywheel | A live red-team against the real model | -tags live -> same attacks against the model itself -> live receipt |
| 13.1 | full-chain-attack-contained | P6 ch13 capstone | A full-chain attack, contained | one poisoned email tries everything -> replay by trace id -> every layer holds |
| 13.2 | tasks-and-reminders | P6 ch13 capstone | Tasks and reminders | feature -> its attack -> its fix (one unit) |
| 13.3 | my-week | P6 ch13 capstone | My week at a glance | feature -> its attack -> its fix (one unit) |
| 13.4 | the-directory-and-contacts | P6 ch13 capstone | The directory and contacts | feature -> PII surfacing attack -> scoped surfacing |
| 13.5 | action-links | P6 ch13 capstone | Action links | HITL + netpolicy + VM fetch -> action-link-exfil -> 0% |
| 13.6 | calendar-crud-and-export | P6 ch13 capstone | Calendar CRUD and export | dup-free export -> duplicate-side-effect -> exactly once |
| 13.7 | the-ui-and-chat | P6 ch13 capstone | The UI and chat | chat over the corpus -> chat-rag-injection -> 0% |
| 14.1 | governed-control-plane | P7 ch14 governance | A governed control plane and operator roles | prompt swap without code -> RBAC + who did what recorded |
| 14.2 | the-agent-gets-a-wallet | P7 ch14 governance | The agent gets a wallet | non-human identity -> revoked-token-still-works -> revocation holds |
| 14.3 | admin-audit-and-rollback | P7 ch14 governance | Admin audit and one-click rollback | bad change -> rollback cleanly -> audited |
| 14.4 | incident-response-and-the-kill-switch | P7 ch14 governance | Incident response and the kill switch | layered stop -> unauthz-killswitch + deny storm -> contained |
| 14.5 | data-and-privacy-governance | P7 ch14 governance | Data and privacy governance | dataset-poisoning + promotion-gate-bypass -> gates hold |
| 14.6 | output-authenticity | P7 ch14 governance | Output authenticity | sign what the agent produces (C2PA) -> output-forgery -> 0% |
| 15.1 | durable-execution | P7 ch15 reliability | Durable execution and resume-verify | resume-tampered-state -> hash-chained steps -> refuse forged resume |
| 15.2 | exactly-once-side-effects | P7 ch15 reliability | Exactly-once side effects | duplicate-side-effect -> idempotency -> once |
| 15.3 | context-compaction | P7 ch15 reliability | Context compaction and summarization injection | summarization-injection -> trusted-summary rules -> 0% |
| 15.4 | agent-memory-architecture | P7 ch15 reliability | Agent memory architecture | registry and blackboard -> memory lifecycle -> scoped recall |
| 16.1 | make-it-idiomatic | P7 ch16 idiomatic | Make it idiomatic: the repo refactor | the refactor -> no attack -> the product as Go you are not embarrassed by |
