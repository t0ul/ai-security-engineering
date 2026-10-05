"""
telemetry.py — the observability spine.

In an agentic system the trace IS the detection surface: it's the only durable
record that an injection was attempted, a policy gate fired, a tool ran with
odd args, or data tried to leave. This module gives every request one
correlation `trace_id` and writes a structured, append-only, hash-chained
audit log so the record is tamper-evident and machine-queryable (by the M11
eval harness and M12 forensic review).

Three trust properties, because "logs are truth" only if the log is trustworthy:
  1. Integrity   — hash-chained JSONL; altering any record breaks the chain.
  2. Redaction   — values are scrubbed before they're written; we log that a
                   secret was present, never the secret itself.
  3. Anti-injection — control chars / newlines in logged strings are defanged,
                   so untrusted email text can't forge or split log lines.

Stdlib only, so the same approach works on the host control plane and (in a
trimmed form) inside the MicroVM daemon. The heavier OpenTelemetry + Langfuse
backbone is the Module 2 upgrade; this is the spine everything hangs on.
"""
import os
import re
import json
import time
import uuid
import hashlib
import threading
import contextlib

# --- value-level redaction: never let a secret/PII value reach the log ---
_REDACT = [
    (re.compile(r"(?i)\b\w*(?:secret|passwd|password|token|apikey|api[_-]?key|access[_-]?key|credential|private[_-]?key)\w*\s*=\s*\S+"), "<SECRET=redacted>"),
    (re.compile(r"\b(?:AKIA|ASIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|A3T[A-Z0-9])[A-Z0-9]{16}\b"), "<AWS_KEY>"),
    (re.compile(r"\b(?:sk|pk|rk|ghp|gho|ghs|xox[baprs])[-_][A-Za-z0-9]{16,}\b"), "<TOKEN>"),
    (re.compile(r"\beyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-]+\b"), "<JWT>"),
    (re.compile(r"(?i)\bbearer\s+[A-Za-z0-9._\-]{10,}\b"), "<BEARER>"),
    (re.compile(r"\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b"), "<EMAIL>"),
]
_CTRL = re.compile(r"[\x00-\x1f\x7f]")  # control chars incl new/CR -> log-injection defense
_MAX = 2000


def _clean(value):
    if isinstance(value, str):
        for rx, repl in _REDACT:
            value = rx.sub(repl, value)
        value = _CTRL.sub(" ", value)           # defang forged log lines
        if len(value) > _MAX:
            value = value[:_MAX] + "…"
        return value
    if isinstance(value, dict):
        return {k: _clean(v) for k, v in value.items()}
    if isinstance(value, (list, tuple)):
        return [_clean(v) for v in value]
    return value


def new_trace_id():
    return uuid.uuid4().hex


def _core(trace_id, span, event, service, fields):
    return {
        "ts": round(time.time(), 3),
        "trace_id": trace_id,
        "service": service,
        "span": span,
        "event": event,
        "fields": _clean(fields),
    }


class AuditLog:
    def __init__(self, path=None, service="control-plane"):
        self.path = path or os.environ.get("AUDIT_LOG") or os.path.join(
            os.path.dirname(os.path.abspath(__file__)), "logs", "audit.jsonl"
        )
        os.makedirs(os.path.dirname(self.path), exist_ok=True)
        self.service = service
        self._lock = threading.Lock()
        self._prev = self._last_hash()

    def _last_hash(self):
        try:
            last = ""
            with open(self.path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if line:
                        last = line
            return json.loads(last).get("hash", "") if last else ""
        except (FileNotFoundError, ValueError):
            return ""

    def emit(self, trace_id, span, event, **fields):
        core = _core(trace_id, span, event, self.service, fields)
        body = json.dumps(core, sort_keys=True, ensure_ascii=False)
        with self._lock:
            prev = self._prev
            digest = hashlib.sha256((prev + body).encode("utf-8")).hexdigest()
            rec = dict(core)
            rec["prev"] = prev
            rec["hash"] = digest
            with open(self.path, "a", encoding="utf-8") as f:
                f.write(json.dumps(rec, ensure_ascii=False) + "\n")
            self._prev = digest
        print(f"🧾 [{span}] {event} trace={trace_id[:8]}", flush=True)
        return digest

    def verify(self):
        """Re-walk the chain. Returns (ok: bool, records_checked: int)."""
        prev = ""
        n = 0
        try:
            with open(self.path, "r", encoding="utf-8") as f:
                for line in f:
                    line = line.strip()
                    if not line:
                        continue
                    rec = json.loads(line)
                    stored_prev = rec.pop("prev", None)
                    stored_hash = rec.pop("hash", None)
                    body = json.dumps(rec, sort_keys=True, ensure_ascii=False)
                    calc = hashlib.sha256((prev + body).encode("utf-8")).hexdigest()
                    if stored_prev != prev or stored_hash != calc:
                        return False, n
                    prev = stored_hash
                    n += 1
        except FileNotFoundError:
            return True, 0
        return True, n


# Process-wide singleton so every module shares one chain/file.
_AUDIT = None
_AUDIT_LOCK = threading.Lock()


def get_audit(service="control-plane"):
    global _AUDIT
    if _AUDIT is None:
        with _AUDIT_LOCK:
            if _AUDIT is None:
                _AUDIT = AuditLog(service=service)
    return _AUDIT


@contextlib.contextmanager
def span(trace_id, name, **start_fields):
    audit = get_audit()
    t0 = time.time()
    audit.emit(trace_id, name, "start", **start_fields)
    status = "ok"
    try:
        yield audit
    except Exception as exc:  # noqa: BLE001
        status = "error"
        audit.emit(trace_id, name, "error", error=str(exc))
        raise
    finally:
        audit.emit(trace_id, name, "end", status=status, ms=round((time.time() - t0) * 1000, 1))
