"""
process_email — the pure, testable core of the watched-folder agent.

Reads one email, runs the registered tools under one trace_id, enforces each
tool's declared capability (a read-only tool cannot write an artifact), writes
inert artifacts to the outbox, and records every step to the audit log. The
watcher.py daemon is a thin wrapper that calls this per dropped file.
"""
import os
import sys

_HERE = os.path.dirname(os.path.abspath(__file__))
# Watcher logs land in emaildrop/logs unless overridden.
os.environ.setdefault("AUDIT_LOG", os.path.join(_HERE, "logs", "audit.jsonl"))
# Share the control-plane telemetry spine.
sys.path.insert(0, os.path.join(_HERE, "..", "control_plane"))
sys.path.insert(0, _HERE)

from telemetry import get_audit, new_trace_id, span      # noqa: E402
from tools.base import Capability                         # noqa: E402
import tools.event_extractor  # noqa: E402,F401  (registers the tool)
from tools.base import all_tools                          # noqa: E402

MAX_BYTES = 256 * 1024  # oversized-input DoS guard (Module 10 / long-running agent)


def process_email(path, outbox_dir=None, tool_names=None, default_year=2026):
    audit = get_audit()
    trace_id = new_trace_id()
    source = os.path.basename(path)
    outbox_dir = outbox_dir or os.path.join(_HERE, "outbox")
    os.makedirs(outbox_dir, exist_ok=True)

    audit.emit(trace_id, "request", "start", source=source)

    size = os.path.getsize(path)
    if size > MAX_BYTES:
        audit.emit(trace_id, "ingest", "rejected", reason="oversized", bytes=size, limit=MAX_BYTES)
        audit.emit(trace_id, "request", "end", status="rejected")
        return {"trace_id": trace_id, "rejected": "oversized", "bytes": size}

    with open(path, encoding="utf-8", errors="replace") as f:
        text = f.read()

    # Module 4: neutralize indirect prompt injection in the untrusted email body,
    # and record what was found (the audit trail is the detection surface).
    try:
        from injection_guard import guard
        text, findings = guard(text)
        if findings:
            audit.emit(trace_id, "injection_guard", "detected", indicators=findings)
    except Exception as e:  # never let the guard crash the pipeline
        audit.emit(trace_id, "injection_guard", "error", error=str(e))

    registry = all_tools()
    names = tool_names or ["event_extractor"]
    ctx = {"source": source, "default_year": default_year}
    stem = os.path.splitext(source)[0]
    summary = {"trace_id": trace_id, "source": source, "events": 0,
               "artifacts": [], "warnings": []}

    for name in names:
        tool = registry.get(name)
        if tool is None:
            continue
        with span(trace_id, name, capability=tool.capability.value):
            result = tool.run(text, ctx)

        # Capability enforcement (least privilege, Module 6): only a WRITE_ICS
        # tool may emit artifacts; anything else is dropped and flagged.
        artifacts = result.artifacts
        if artifacts and result.capability != Capability.WRITE_ICS:
            audit.emit(trace_id, name, "capability_violation",
                       declared=result.capability.value, tried_to_write=list(artifacts))
            artifacts = {}

        for fname, content in artifacts.items():
            out_path = os.path.join(outbox_dir, f"{stem}.{fname}")
            with open(out_path, "w", encoding="utf-8") as f:
                f.write(content)
            summary["artifacts"].append(out_path)
            audit.emit(trace_id, name, "artifact_written", file=os.path.basename(out_path),
                       bytes=len(content))

        summary["events"] += len(result.events)
        summary["warnings"] += result.warnings
        audit.emit(trace_id, name, "result", events=len(result.events),
                   warnings=result.warnings)

    audit.emit(trace_id, "request", "end", status="ok",
               events=summary["events"], artifacts=len(summary["artifacts"]))
    return summary


if __name__ == "__main__":
    import json
    target = sys.argv[1] if len(sys.argv) > 1 else os.path.join(_HERE, "samples", "3.txt")
    print(json.dumps(process_email(target), indent=2))
