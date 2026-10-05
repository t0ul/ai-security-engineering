"""Event schema — the typed contract every calendar tool produces."""
from dataclasses import dataclass, asdict, field
from typing import Optional, List

_ALLOWED = {"title", "start", "end", "all_day", "location", "source_email", "confidence", "warnings"}


@dataclass
class Event:
    title: str
    start: str                       # ISO 8601: "2026-09-24T08:30:00" (or "2026-09-24" if all_day)
    end: Optional[str] = None        # ISO 8601 or None
    all_day: bool = False
    location: Optional[str] = None
    source_email: Optional[str] = None
    confidence: float = 1.0          # 0..1; low-confidence events get flagged for human review
    warnings: List[str] = field(default_factory=list)  # e.g. "weekday_mismatch: stated Thu, 2026-09-29 is Tue"

    def to_dict(self) -> dict:
        return asdict(self)

    @staticmethod
    def from_dict(d: dict) -> "Event":
        return Event(**{k: v for k, v in d.items() if k in _ALLOWED})
