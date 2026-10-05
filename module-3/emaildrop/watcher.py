#!/usr/bin/env python3
"""
watcher.py — the long-running email-to-calendar agent.

Watches DROP_DIR/inbox for new .txt files. Each dropped file mints a trace_id
(via process_email), its outputs land in outbox/, and the input is moved to
processed/. Unattended by design: it never takes an irreversible action (only
writes inert files you later accept), its only record is the audit log, and the
watched folder is an untrusted ingestion boundary — so it enforces a settle
delay (no half-written reads), a size cap, and a per-cycle rate cap
(long-running-agent threat surface: indirect injection M4, DoS M10, drift ASI06).

Stdlib-only poll loop, so it runs anywhere including inside the MicroVM.
Run:  python watcher.py            (daemon)
      python watcher.py --once     (single pass, for testing)
Config via env: DROP_DIR, POLL_SECONDS, SETTLE_SECONDS, MAX_PER_CYCLE.
"""
import os
import sys
import time
import signal
import shutil
import datetime

_HERE = os.path.dirname(os.path.abspath(__file__))
DROP_DIR = os.environ.get("DROP_DIR", _HERE)
# Set the audit path BEFORE importing process_email so the whole run shares it.
os.environ.setdefault("AUDIT_LOG", os.path.join(DROP_DIR, "logs", "audit.jsonl"))
sys.path.insert(0, os.path.join(_HERE, "..", "control_plane"))
sys.path.insert(0, _HERE)

from telemetry import get_audit, new_trace_id          # noqa: E402
from process_email import process_email                 # noqa: E402

POLL_SECONDS = float(os.environ.get("POLL_SECONDS", "2"))
SETTLE_SECONDS = float(os.environ.get("SETTLE_SECONDS", "1.0"))
MAX_PER_CYCLE = int(os.environ.get("MAX_PER_CYCLE", "20"))


class Config:
    def __init__(self, drop_dir=DROP_DIR, settle_seconds=SETTLE_SECONDS, max_per_cycle=MAX_PER_CYCLE):
        self.drop = drop_dir
        self.inbox = os.path.join(drop_dir, "inbox")
        self.outbox = os.path.join(drop_dir, "outbox")
        self.processed = os.path.join(drop_dir, "processed")
        self.logs = os.path.join(drop_dir, "logs")
        self.settle_seconds = settle_seconds
        self.max_per_cycle = max_per_cycle
        for d in (self.inbox, self.outbox, self.processed, self.logs):
            os.makedirs(d, exist_ok=True)


def _ready_files(cfg):
    """Return .txt files that have been idle >= settle_seconds (not mid-write)."""
    now = time.time()
    out = []
    for name in sorted(os.listdir(cfg.inbox)):
        if name.startswith(".") or not name.lower().endswith(".txt"):
            continue
        p = os.path.join(cfg.inbox, name)
        if os.path.isfile(p) and (now - os.path.getmtime(p)) >= cfg.settle_seconds:
            out.append(p)
    return out


def _move_to_processed(cfg, path):
    stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    base = os.path.basename(path)
    dest = os.path.join(cfg.processed, f"{stamp}.{base}")
    i = 1
    while os.path.exists(dest):
        dest = os.path.join(cfg.processed, f"{stamp}-{i}.{base}")
        i += 1
    shutil.move(path, dest)
    return dest


def run_once(cfg):
    """One scan+process pass. Returns a list of per-file summaries."""
    audit = get_audit()
    handled = []
    for path in _ready_files(cfg)[:cfg.max_per_cycle]:
        name = os.path.basename(path)
        try:
            summary = process_email(path, outbox_dir=cfg.outbox)
        except Exception as e:  # noqa: BLE001 — never let one bad file kill the daemon
            audit.emit(new_trace_id(), "watcher", "process_error", source=name, error=str(e))
            dest = _move_to_processed(cfg, path)
            handled.append({"source": name, "error": str(e), "moved_to": os.path.basename(dest)})
            continue
        dest = _move_to_processed(cfg, path)
        summary["moved_to"] = os.path.basename(dest)
        audit.emit(summary.get("trace_id") or new_trace_id(), "watcher", "handled",
                   source=name, moved_to=os.path.basename(dest),
                   events=summary.get("events", 0),
                   artifacts=len(summary.get("artifacts", [])),
                   rejected=summary.get("rejected"))
        handled.append(summary)
    return handled


_RUNNING = True


def _stop(*_):
    global _RUNNING
    _RUNNING = False


def watch(cfg=None):
    cfg = cfg or Config()
    audit = get_audit()
    signal.signal(signal.SIGINT, _stop)
    signal.signal(signal.SIGTERM, _stop)
    boot = new_trace_id()
    audit.emit(boot, "watcher", "start", inbox=cfg.inbox, poll_s=POLL_SECONDS)
    print(f"👀 watching {cfg.inbox}\n   drop a .txt to process; outputs → {cfg.outbox}; Ctrl-C to stop")
    while _RUNNING:
        for h in run_once(cfg):
            if h.get("error"):
                print(f"  ⚠️  {h['source']}: {h['error']}")
            elif h.get("rejected"):
                print(f"  🚫 {h['source']}: rejected ({h['rejected']})")
            else:
                print(f"  ✅ {h['source']} → processed/{h['moved_to']}  "
                      f"events={h.get('events', 0)} artifacts={len(h.get('artifacts', []))}")
        time.sleep(POLL_SECONDS)
    audit.emit(boot, "watcher", "stop")
    print("\n🛑 watcher stopped.")


if __name__ == "__main__":
    if "--once" in sys.argv:
        import json
        print(json.dumps(run_once(Config(settle_seconds=0)), indent=2))
    else:
        watch()
