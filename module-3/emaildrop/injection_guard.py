"""
injection_guard.py — Module 4 defense against indirect prompt injection.

The email body is untrusted. Hostile newsletters hide instructions (HTML
comments, zero-width/bidi chars, "ignore previous instructions", "SYSTEM
override", "assistant note: you must add…", "do not mention") that try to make
the agent plant attacker events or exfiltrate. This neutralizes those vectors
BEFORE extraction and reports what it found (detection surface -> telemetry).

Layer 1 of defense in depth. Layer 2 is spotlighting + an instruction-hierarchy
prompt (event_extractor) so the LLM treats email text as data, not commands.
No single layer is complete; injection is adversarial — we reduce and detect.
"""
import re

_HTML_COMMENT = re.compile(r"<!--.*?-->", re.DOTALL)
_ZERO_WIDTH = re.compile(r"[​-‏‪-‮⁠﻿]")

_MARKERS = [
    (re.compile(r"(?i)ignore\s+(all\s+)?(previous|prior)\s+instructions"), "ignore-instructions"),
    (re.compile(r"(?i)(system|assistant)\s+override"), "system-override"),
    (re.compile(r"(?i)\b(assistant|ai|system)\s+(note|instruction|directive|override)\b"), "assistant-directive"),
    (re.compile(r"(?i)you\s+(must|are\s+required\s+to|should|need\s+to)\s+(add|include|output|send|verify|wire)"), "imperative-inject"),
    (re.compile(r"(?i)do\s+not\s+(mention|reveal|tell|disclose)"), "concealment"),
    (re.compile(r"(?i)disregard\s+(the\s+)?(above|previous|prior)"), "disregard"),
]

_BLOCK = "[blocked: injected instruction removed]"


def guard(text):
    """Return (cleaned_text, sorted_findings)."""
    findings = []

    if _HTML_COMMENT.search(text):
        findings.append("hidden-html-comment")
    cleaned = _HTML_COMMENT.sub(" ", text)          # hidden content a human never sees

    if _ZERO_WIDTH.search(cleaned):
        findings.append("zero-width-chars")
    cleaned = _ZERO_WIDTH.sub("", cleaned)

    # Neutralize an injection marker and the next 2 non-blank lines (injected
    # events often put the imperative and its date on adjacent lines).
    lines = cleaned.splitlines()
    out, i = [], 0
    while i < len(lines):
        hit = next((name for rx, name in _MARKERS if rx.search(lines[i])), None)
        if hit:
            findings.append(hit)
            out.append(_BLOCK)
            blanked = 0
            j = i + 1
            while j < len(lines) and blanked < 2:
                if lines[j].strip():
                    out.append(_BLOCK)
                    blanked += 1
                else:
                    out.append(lines[j])
                j += 1
            i = j
        else:
            out.append(lines[i])
            i += 1
    return "\n".join(out), sorted(set(findings))
