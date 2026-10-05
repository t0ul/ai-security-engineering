"""
Deterministic date/time parsing + integrity checking (Module 9).

School emails bury dates in prose, omit the year, and sometimes state the WRONG
weekday (the real corpus lists Back-to-School Night as both Tuesday and Thursday
Sept 29). We never trust an LLM's date arithmetic: a model may propose the text
span, but normalization and the weekday-vs-date consistency check are done here,
deterministically, in stdlib.
"""
import re
import datetime

MONTHS = {m.lower(): i for i, m in enumerate(
    ["January", "February", "March", "April", "May", "June",
     "July", "August", "September", "October", "November", "December"], start=1)}
WEEKDAYS = ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"]
_WD_INDEX = {w.lower(): i for i, w in enumerate(WEEKDAYS)}

_RE_WEEKDAY = re.compile(r"(?i)\b(" + "|".join(WEEKDAYS) + r")\b")
_RE_MONTHDAY = re.compile(r"(?i)\b(" + "|".join(MONTHS) + r")\s+(\d{1,2})(?:st|nd|rd|th)?\b")
_RE_YEAR = re.compile(r"\b(20\d{2})\b")
_RE_TIME = re.compile(r"(?i)\b(\d{1,2})(?::(\d{2}))?\s*([ap]\.?m\.?)")


def _norm_ampm(s: str) -> str:
    return "pm" if s.lower().startswith("p") else "am"


def parse_times(text: str):
    """Return [(hour24, minute), ...] in order of appearance."""
    out = []
    for h, m, ap in _RE_TIME.findall(text):
        hour = int(h) % 12
        if _norm_ampm(ap) == "pm":
            hour += 12
        out.append((hour, int(m) if m else 0))
    return out


def parse_date_phrase(text: str, default_year: int = 2026):
    """
    Pull month/day (+ optional stated weekday, optional explicit year) from a phrase.
    Returns a dict with the resolved date and an integrity flag, or None if no date.
    """
    md = _RE_MONTHDAY.search(text)
    if not md:
        return None
    month = MONTHS[md.group(1).lower()]
    day = int(md.group(2))
    ym = _RE_YEAR.search(text)
    year = int(ym.group(1)) if ym else default_year
    try:
        d = datetime.date(year, month, day)
    except ValueError:
        return None

    stated = _RE_WEEKDAY.search(text)
    stated_wd = stated.group(1).title() if stated else None
    actual_wd = WEEKDAYS[d.weekday()]
    mismatch = bool(stated_wd) and stated_wd != actual_wd
    warning = (f"weekday_mismatch: stated {stated_wd}, but {d.isoformat()} is {actual_wd}"
               if mismatch else None)
    return {
        "date": d,
        "stated_weekday": stated_wd,
        "actual_weekday": actual_wd,
        "weekday_mismatch": mismatch,
        "warning": warning,
        "assumed_year": ym is None,
    }


def extract_datetime(phrase: str, default_year: int = 2026):
    """
    High-level: phrase -> {start, end, all_day, warnings}.
    start/end are ISO strings; all_day when no time is present.
    """
    dp = parse_date_phrase(phrase, default_year)
    if not dp:
        return None
    d = dp["date"]
    times = parse_times(phrase)
    warnings = []
    if dp["warning"]:
        warnings.append(dp["warning"])
    if dp["assumed_year"]:
        warnings.append(f"assumed_year:{d.year}")

    if not times:
        return {"start": d.isoformat(), "end": None, "all_day": True, "warnings": warnings}

    sh, sm = times[0]
    start = datetime.datetime(d.year, d.month, d.day, sh, sm).isoformat()
    end = None
    if len(times) > 1:
        eh, em = times[1]
        end = datetime.datetime(d.year, d.month, d.day, eh, em).isoformat()
    return {"start": start, "end": end, "all_day": False, "warnings": warnings}
