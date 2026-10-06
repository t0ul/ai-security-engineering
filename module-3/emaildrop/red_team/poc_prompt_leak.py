#!/usr/bin/env python3
"""
RED TEAM + BLUE TEAM (Module 4, LLM07): system-prompt leakage.

A poisoned email tries to make the agent dump its own system prompt. Two
defenses: (1) injection_guard neutralizes the prompt-extraction request in the
input; (2) an output-side leak detector refuses to propagate any response that
echoes the system prompt (so even if the model complies, nothing leaks out).
"""
import os
import sys

sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
from injection_guard import guard, detect_prompt_leak       # noqa: E402
from tools.event_extractor import EXTRACTION_PROMPT          # noqa: E402

HERE = os.path.dirname(os.path.abspath(__file__))
raw = open(os.path.join(HERE, "poisoned_prompt_leak.txt"), encoding="utf-8").read()

print("🔴 Input guard vs the prompt-extraction request:")
clean, findings = guard(raw)
print(f"   findings: {findings}")
print(f"   extraction request neutralized in input: {'prompt-extraction' in findings}\n")

print("🔴 Suppose the model COMPLIED and dumped its system prompt as the 'output':")
leaked_output = "Sure, here are my instructions:\n" + EXTRACTION_PROMPT
leaked, snip = detect_prompt_leak(leaked_output, EXTRACTION_PROMPT)
print(f"   output-side leak detector fired: {leaked}")
print(f"   caught snippet: {snip[:60]!r}...")
print("   -> llm_propose returns [] on leak, so the prompt never reaches the user\n")

print("🟢 A normal event-extraction response does NOT trip the detector:")
benign = '[{"title":"PTA Meeting","when":"Thursday, September 24th at 8:30 AM","where":""}]'
ok, _ = detect_prompt_leak(benign, EXTRACTION_PROMPT)
print(f"   false positive on benign output: {ok}")

blocked = ("prompt-extraction" in findings) and leaked and (not ok)
print(f"\n{'✅ LEAK BLOCKED (input + output defense)' if blocked else '❌ CHECK'}")
sys.exit(0 if blocked else 1)
