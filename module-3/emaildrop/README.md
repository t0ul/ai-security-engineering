# Email-to-Calendar Agent (Module 3 vertical slice)

A local, security-hardened agent that turns messy emails (e.g. school newsletters)
into calendar events you accept by hand. Drop a `.txt` into `inbox/`; an inert
`.ics` appears in `outbox/` to double-click into Apple Calendar.

This is tool #1 of a planner-over-scoped-tools agent. It runs entirely on the host
control plane; untrusted command execution (and, later, tool calls) detonate in the
Apple `vz` MicroVM reached over vsock — the control plane never runs untrusted code.

## Layout

```
emaildrop/
  inbox/        drop email .txt here (untrusted ingestion boundary)
  outbox/       agent writes <name>.events.ics (double-click to accept)
  processed/    inputs moved here after handling (timestamped)
  logs/         audit.jsonl — hash-chained, auto-redacting trace log
  samples/      anonymized example emails (real data is git-ignored)
  labels/       hand-labeled ground truth for the eval harness
  tools/        tool plugins (event_extractor = tool #1)
  schema.py     the Event contract
  dateparse.py  deterministic date/time parsing + weekday integrity check
  icswriter.py  inert .ics generation + field sanitizer
  process_email.py  pure core: run tools under one trace_id, enforce capability, write outputs
  watcher.py    long-running daemon: watch inbox/, fire process_email per file
  eval.py       score extraction against labels/ (regression metric)
```

## Prerequisites (host control plane)

Same stack as the rest of the project:
- `llama.cpp` serving the planner (`:11435`) and coder (`:11436`) GGUFs
- LiteLLM gateway on `:4000` (aliases `planner`, `coder`)
- the `.venv` control-plane environment (`requests`, `presidio`, `langgraph`, …)
- for real detonation, the MicroVM booted (`set-up/launch_vm.go`) — not required for
  calendar extraction, which does no shell execution

## Run

```sh
# one-shot (process a single file)
python process_email.py samples/3.txt

# long-running daemon (drop files into inbox/ while it runs)
python watcher.py
#   point it at a different folder:  DROP_DIR=~/EmailDrop python watcher.py

# extraction mode: auto (LLM if gateway reachable, else regex) | llm | regex
EXTRACT_MODE=auto python watcher.py
```

Accept step: open the generated `outbox/<name>.events.ics` — macOS Calendar's import
dialog is the human-in-the-loop gate. The agent never writes to Calendar itself.

## Evaluate (Module 11 loop)

```sh
python eval.py labels/3.json           # deterministic regex path
EXTRACT_MODE=llm python eval.py labels/1.json --tool   # score the real model
```

Current numbers: `3.txt` F1 = 1.00; `1.txt` regex baseline F1 = 0.12, local-3B LLM
path F1 = 0.87 (precision 1.00, recall 0.77 — misses all-day "days off" and one
buried training; a prompt-calibration target).

## Security properties demonstrated

- **Isolation:** control plane on host; untrusted execution confined to the vz
  MicroVM over inbound-only vsock.
- **Observability = detection:** every step emits a span under one `trace_id` to a
  **hash-chained** audit log that **auto-redacts** secrets/PII and defangs
  log-injection. One id reconstructs (and verifies) a whole request.
- **Least privilege:** each tool declares a capability; `process_email` refuses to
  write artifacts from a read-only tool.
- **Output handling:** the `.ics` is inert; the field sanitizer strips URLs/active
  schemes (exfil) and escapes per RFC 5545 (calendar line-injection).
- **Data integrity:** dates are resolved deterministically (the LLM proposes a span,
  the code decides the date) with a weekday-vs-date mismatch check.
- **Ingestion hardening:** settle delay (no half-written reads), size cap, per-cycle
  rate cap; one bad file never kills the daemon.
- **Privacy posture:** real emails are git-ignored; the repo ships anonymized samples;
  logs never record PII values. (The email body is intentionally NOT redacted before
  extraction, to preserve event titles.)

## Known gaps / next

- Recall calibration: add few-shot examples for all-day days-off and buried trainings.
- Labels for the remaining samples (2,4–7) to harden the extractor.
- Tool #2: action-items / deadlines (read-only), behind the same plugin contract.
- Injection guardrails for the untrusted email body → Module 4.
- Expose tools as scoped MCP tools + A2A auth → Module 7.
