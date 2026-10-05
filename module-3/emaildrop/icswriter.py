"""
Inert .ics generation + field sanitizer (Module 5: improper output handling).

The .ics is an inert artifact the user accepts by double-click — it executes
nothing. But event fields are an exfiltration surface (a URL in NOTES/LOCATION
can beacon when a client renders it), so we strip links/active schemes from
every field, and escape per RFC 5545 so content can't break the line structure
(the calendar analog of the markdown-image sanitizer).
"""
import re
import uuid
import datetime

_URL = re.compile(r"(?i)\b(?:https?|ftp)://\S+")
_SCHEME = re.compile(r"(?i)\b(?:javascript|data|file|vbscript):\S+")


def sanitize_field(value: str):
    """Strip exfil vectors from a field; return (clean, removed_count)."""
    if not value:
        return "", 0
    removed = len(_URL.findall(value)) + len(_SCHEME.findall(value))
    value = _URL.sub("[link removed]", value)
    value = _SCHEME.sub("[blocked]", value)
    return value, removed


def _escape(value: str) -> str:
    # RFC 5545 text escaping: backslash, semicolon, comma, newlines.
    return (value.replace("\\", "\\\\")
                 .replace(";", "\\;")
                 .replace(",", "\\,")
                 .replace("\r\n", "\\n").replace("\n", "\\n").replace("\r", "\\n"))


def _dt(value: str, all_day: bool) -> str:
    if all_day:
        d = datetime.date.fromisoformat(value[:10])
        return d.strftime("%Y%m%d")
    dt = datetime.datetime.fromisoformat(value)
    return dt.strftime("%Y%m%dT%H%M%S")


def write_ics(events, calname: str = "Email-to-Calendar"):
    """events: list[Event]. Returns (ics_text, total_links_removed)."""
    now = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    removed_total = 0
    lines = ["BEGIN:VCALENDAR", "VERSION:2.0",
             "PRODID:-//ai-security-engineering//email-to-calendar//EN",
             f"X-WR-CALNAME:{_escape(calname)}"]
    for ev in events:
        title, r1 = sanitize_field(ev.title or "Untitled")
        location, r2 = sanitize_field(ev.location or "")
        removed_total += r1 + r2
        lines.append("BEGIN:VEVENT")
        lines.append(f"UID:{uuid.uuid4().hex}@email-to-calendar")
        lines.append(f"DTSTAMP:{now}")
        if ev.all_day:
            lines.append(f"DTSTART;VALUE=DATE:{_dt(ev.start, True)}")
        else:
            lines.append(f"DTSTART:{_dt(ev.start, False)}")
            if ev.end:
                lines.append(f"DTEND:{_dt(ev.end, False)}")
        lines.append(f"SUMMARY:{_escape(title)}")
        if location:
            lines.append(f"LOCATION:{_escape(location)}")
        if ev.warnings:
            desc, r3 = sanitize_field(" | ".join(ev.warnings))
            removed_total += r3
            lines.append(f"DESCRIPTION:{_escape('⚠ ' + desc)}")
        lines.append("END:VEVENT")
    lines.append("END:VCALENDAR")
    return "\r\n".join(lines) + "\r\n", removed_total
