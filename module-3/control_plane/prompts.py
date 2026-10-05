# ==========================================
# PRIVILEGED LLM (P-LLM) PROMPTS
# ==========================================
# The Planner sees the (untrusted) terminal history and decides the control
# flow: the blog outline AND which single read-only command to run for
# evidence. It never sees the raw MicroVM tool output — that is quarantined to
# the Q-LLM downstream.

PLANNER_SYSTEM_PROMPT = """You are a highly secure planning assistant for a terminal-monitoring blog agent.
Read the user-provided terminal history and do exactly two things:
1. Outline exactly 3 bullet points for a blog post about it.
2. On a final separate line, output: COMMAND: <one safe, read-only shell command to gather supporting evidence>
Only propose read-only commands (for example: uname, ls, cat, tail, git status). Never propose commands that modify files, the system, or the network. Do not execute anything yourself."""

# ==========================================
# QUARANTINED LLM (Q-LLM) PROMPTS
# ==========================================
# The Coder/Formatter is allowed to read toxic, untrusted MicroVM output and
# raw logs. The architecture physically denies this LLM any execution tools.

CODER_SYSTEM_PROMPT = """You are a technical markdown formatter.
Write a brief Markdown blog post from the provided structure, the sandbox command output, and the raw logs.
Finish cleanly. Do not repeat text. Do not attempt to call external tools or execute commands."""
