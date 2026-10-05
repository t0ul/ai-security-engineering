"""
Tool #1 — Event extractor (capability: WRITE_ICS).

propose candidates (deterministic here; the LLM planner is the production
proposer) -> validate/normalize dates (dateparse, incl. weekday integrity) ->
merge mentions by date -> inert .ics.
"""
import re
import sys
import os

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from schema import Event
from dateparse import extract_datetime, _RE_MONTHDAY
from tools.base import Tool, Capability, ToolResult, register
from icswriter import write_ics
import json
import urllib.request

GATEWAY_URL = os.environ.get("GATEWAY_URL", "http://localhost:4000/v1/chat/completions")
PLANNER_MODEL = os.environ.get("PLANNER_MODEL", "planner")
EXTRACT_MODE = os.environ.get("EXTRACT_MODE", "auto")  # auto | llm | regex

EXTRACTION_PROMPT = (
    "You extract calendar events from a school newsletter.\n"
    "Return ONLY a JSON array. Each item: "
    '{"title": "<short event name>", "when": "<the date/time EXACTLY as written, '
    'including the weekday if present>", "where": "<location if stated, else empty>"}.\n'
    "Include every dated, actionable event: meetings, drills, days off / half days, "
    "deadlines, trainings, visits, performances.\n"
    "Do NOT include references to PAST events, routine daily arrival/dismissal times, "
    "or general informational dates.\n"
    "Copy each date/time phrase verbatim from the text; never compute or reformat a date.\n"
    "ALWAYS include no-school days, half days, and holidays even with NO time — they are "
    "all-day events and are the easiest to miss. Examples:\n"
    '  "Monday, October 12th- Italian Heritage Day" -> {"title": "Italian Heritage Day (No School)", "when": "Monday, October 12th", "where": ""}\n'
    '  "volunteer training session on Friday, October 9th at 8:45 AM" -> {"title": "Volunteer Training", "when": "Friday, October 9th at 8:45 AM", "where": ""}'
)


def _parse_candidates(content):
    i, j = content.find("["), content.rfind("]")
    if i == -1 or j == -1:
        return []
    try:
        arr = json.loads(content[i:j + 1])
    except Exception:
        return []
    out = []
    for it in arr:
        if isinstance(it, dict) and it.get("when"):
            out.append({"title": str(it.get("title", "")).strip(), "phrase": str(it["when"]).strip(), "location": str(it.get("where", "")).strip()})
    return out


def llm_propose(email_text, timeout=90):
    """Ask the planner model for (title, when) candidates. The LLM proposes spans;
    dateparse decides the actual date, so the model cannot hallucinate a wrong date."""
    payload = {
        "model": PLANNER_MODEL, "temperature": 0.1, "max_tokens": 900,
        "messages": [
            {"role": "system", "content": EXTRACTION_PROMPT},
            {"role": "user", "content": email_text[:12000]},
        ],
    }
    req = urllib.request.Request(GATEWAY_URL, data=json.dumps(payload).encode("utf-8"),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req, timeout=timeout) as r:
        data = json.loads(r.read().decode("utf-8"))
    return _parse_candidates(data["choices"][0]["message"]["content"])


def candidates_to_events(cands, source=None, default_year=2026):
    """Validate LLM-proposed phrases deterministically; merge by date."""
    groups = {}
    for c in cands:
        dt = extract_datetime(c.get("phrase", ""), default_year)
        if not dt:
            continue
        k = dt["start"][:10]
        g = groups.setdefault(k, {"titles": [], "dts": [], "warns": set(), "locs": []})
        if c.get("title"):
            g["titles"].append(c["title"])
        if c.get("location"):
            g["locs"].append(c["location"])
        g["dts"].append(dt)
        for w in dt["warnings"]:
            g["warns"].add(w)
    events = []
    for k, g in sorted(groups.items()):
        timed = [d for d in g["dts"] if not d["all_day"]]
        chosen = timed[0] if timed else g["dts"][0]
        title = min(g["titles"], key=len) if g["titles"] else "(untitled)"
        warns = sorted(w for w in g["warns"] if not w.startswith("assumed_year"))
        conf = round(0.85 - (0.3 if warns else 0.0), 2)
        events.append(Event(title=title, start=chosen["start"], end=chosen["end"],
                            all_day=chosen["all_day"], location=(g["locs"][0] if g["locs"] else None),
                            source_email=source, confidence=conf, warnings=warns))
    return events


_VENUES = ["gymnatorium", "cafeteria", "library", "auditorium", "art room",
           "school building", "gym", "yard", "room 205"]
_COLON = re.compile(r"(?i)^[\s\u25cf\u25cb\u2022*\-\u2013\u2014]*([A-Za-z][^:]{2,60}?):\s*(.+)$")
_WD = re.compile(r"(?i)\b(Monday|Tuesday|Wednesday|Thursday|Friday|Saturday|Sunday)\b")
_STOP = re.compile(r"(?i)\b(on|for|our in person|please also mark your calendars for|we will|join us for|beloved community event)\b")


def _clean_title(t: str) -> str:
    t = t.strip(" \t\u25cf\u25cb\u2022*-\u2013\u2014:")
    t = _STOP.sub("", t)
    t = re.sub(r"\s{2,}", " ", t).strip(" -:,")
    return t


def _venue_near(lines, idx):
    for j in (idx, idx + 1, idx + 2):
        if 0 <= j < len(lines):
            low = lines[j].lower()
            for v in _VENUES:
                if v in low:
                    return lines[j].strip(" \t\u25cf\u25cb\u2022*-\u2013\u2014").strip()[:60]
    return None


def propose_candidates(text: str):
    lines = text.splitlines()
    cands = []
    for i, line in enumerate(lines):
        if not _RE_MONTHDAY.search(line):
            continue
        m = _COLON.match(line)
        if m and _RE_MONTHDAY.search(m.group(2)):
            cands.append({"title": _clean_title(m.group(1)), "phrase": m.group(2),
                          "strong": True, "line": line, "idx": i})
        else:
            marks = [x.start() for x in (_WD.search(line), _RE_MONTHDAY.search(line)) if x]
            title = _clean_title(line[:min(marks)]) if marks else ""
            cands.append({"title": title, "phrase": line, "strong": False,
                          "line": line, "idx": i})
    return cands, lines


def extract_events(text: str, source=None, default_year: int = 2026):
    cands, lines = propose_candidates(text)
    groups = {}
    for c in cands:
        dt = extract_datetime(c["phrase"], default_year)
        if not dt:
            continue
        k = dt["start"][:10]
        g = groups.setdefault(k, {"dts": [], "titles": [], "warns": set(), "loc": None})
        g["dts"].append(dt)
        if c["title"]:
            g["titles"].append((c["strong"], len(c["title"]), c["title"]))
        for w in dt["warnings"]:
            g["warns"].add(w)
        if not g["loc"]:
            g["loc"] = _venue_near(lines, c["idx"])

    events = []
    for k, g in sorted(groups.items()):
        if not g["titles"]:
            continue
        timed = [d for d in g["dts"] if not d["all_day"]]
        chosen = timed[0] if timed else g["dts"][0]
        g["titles"].sort(key=lambda t: (not t[0], t[1]))  # strong first, then shortest
        strong_group = any(t[0] for t in g["titles"])
        title = g["titles"][0][2]
        # Reject date *references* in prose (e.g. "refer to the email sent on September 18"):
        # a weak, sentence-like title is almost never an event name.
        if not strong_group and (len(title.split()) >= 7 or ". " in title):
            continue
        warns = sorted(w for w in g["warns"] if not w.startswith("assumed_year"))
        conf = 0.9 if any(t[0] for t in g["titles"]) else 0.6
        if warns:
            conf = round(conf - 0.3, 2)
        events.append(Event(title=title, start=chosen["start"], end=chosen["end"],
                            all_day=chosen["all_day"], location=g["loc"],
                            source_email=source, confidence=conf, warnings=warns))
    return events



_DAYOFF_HDR = re.compile(r"(?i)(days off|no school|half[\s-]?day|school closed|holiday)")


def extract_days_off(text, source=None, default_year=2026):
    """
    Deterministic, PRECISE pass for the 'days off / half days' section — NYC DOE
    closures rarely match federal holidays, so these high-value all-day events
    must never be dropped. Scoped to the section so it adds no prose false-positives.
    """
    lines = text.splitlines()
    events = []
    i, n = 0, len(lines)
    while i < n:
        if _DAYOFF_HDR.search(lines[i]) and not _RE_MONTHDAY.search(lines[i]):
            j, blanks = i + 1, 0
            while j < n:
                ln = lines[j].strip()
                if not ln:
                    blanks += 1
                    if blanks >= 3:
                        break
                    j += 1
                    continue
                md = _RE_MONTHDAY.search(ln)
                if not md:
                    break  # first non-date, non-blank line ends the block
                blanks = 0
                dt = extract_datetime(ln, default_year)
                trailer = ln[md.end():]
                m = re.match(r"^[\s\-\u2013\u2014:]*(.+)$", trailer)
                title = _clean_title(m.group(1)) if m else ""
                title = re.split(r"\.\s", title)[0].strip()  # first clause only
                if dt and title:
                    warns = [w for w in dt["warnings"] if not w.startswith("assumed_year")]
                    events.append(Event(title=title, start=dt["start"], end=dt["end"],
                                        all_day=dt["all_day"], source_email=source,
                                        confidence=0.8, warnings=warns))
                j += 1
            i = j
        else:
            i += 1
    return events



class EventExtractor(Tool):
    name = "event_extractor"
    capability = Capability.WRITE_ICS

    def run(self, email_text: str, ctx: dict) -> ToolResult:
        source = ctx.get("source")
        year = ctx.get("default_year", 2026)
        mode = ctx.get("mode") or EXTRACT_MODE
        warnings = []
        events = []
        used = "regex"
        if mode in ("auto", "llm"):
            try:
                cands = llm_propose(email_text)
                if cands:
                    events = candidates_to_events(cands, source=source, default_year=year)
                    used = "llm"
            except Exception as e:  # gateway down / bad response
                warnings.append(f"llm proposer unavailable ({e}); regex fallback")
        if not events and mode != "llm":
            events = extract_events(email_text, source=source, default_year=year)
            used = "regex"
        warnings.insert(0, f"extractor_mode={used}")
        # Guarantee the high-value days-off / half-days (NYC closures) even if the
        # LLM skipped them; union by date so precision is preserved.
        have = {e.start[:10] for e in events}
        added = 0
        for e in extract_days_off(email_text, source=source, default_year=year):
            if e.start[:10] not in have:
                events.append(e)
                have.add(e.start[:10])
                added += 1
        events.sort(key=lambda e: e.start)
        if added:
            warnings.append(f"days-off safety-net added {added} event(s)")
        ics, removed = write_ics(events)
        if removed:
            warnings.append(f"sanitizer removed {removed} link(s) from event fields")
        for ev in events:
            if ev.warnings:
                warnings.append(f"{ev.title}: {'; '.join(ev.warnings)}")
        return ToolResult(
            tool="event_extractor", capability=Capability.WRITE_ICS,
            events=events, artifacts={"events.ics": ics}, warnings=warnings)


EVENT_EXTRACTOR = register(EventExtractor())  # register an INSTANCE
