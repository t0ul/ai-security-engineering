import sys
import time
import threading
import requests
import re
from typing import TypedDict
from langgraph.graph import StateGraph, END
from prompts import PLANNER_SYSTEM_PROMPT, CODER_SYSTEM_PROMPT
from telemetry import get_audit, span

# 1. Circuit Breaker Constant
MAX_SESSION_STEPS = 5

class AgentState(TypedDict):
    trace_id: str
    raw_history: str
    blog_plan: str
    tool_command: str
    tool_output: str
    final_markdown: str
    step_count: int

def fetch_with_spinner(url, payload, message):
    done = False
    def spinner():
        chars = ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏']
        i = 0
        while not done:
            sys.stdout.write(f'\r{chars[i % len(chars)]} {message} ')
            sys.stdout.flush()
            time.sleep(0.1)
            i += 1
    t = threading.Thread(target=spinner)
    t.start()
    try:
        response = requests.post(url, json=payload, timeout=120)
        if response.status_code == 400:
            sys.stdout.write('\r' + ' ' * (len(message) + 4) + '\r')
            print(f"\n❌ 400 Bad Request from {url}\nServer Error Log: {response.text}\n")
        response.raise_for_status()
        result = response.json()
    finally:
        done = True
        t.join()
        sys.stdout.write('\r' + ' ' * (len(message) + 4) + '\r')
        sys.stdout.flush()
    return result

def planner_agent(state: AgentState):
    tid = state.get("trace_id", "")
    audit = get_audit()

    # Circuit Breaker Enforcement
    if state.get("step_count", 0) >= MAX_SESSION_STEPS:
        print("\n🛑 [Circuit Breaker] MAX_SESSION_STEPS reached. Halting to prevent DoS/Sponge attack.")
        audit.emit(tid, "planner", "circuit_breaker_tripped", step_count=state.get("step_count", 0))
        return {"blog_plan": "EXECUTION_HALTED"}

    print("🧠 [P-LLM] Llama-3.2 (Planner) analyzing trusted instructions...")
    url = "http://localhost:4000/v1/chat/completions"
    payload = {
        "model": "planner",
        "messages": [
            {"role": "system", "content": PLANNER_SYSTEM_PROMPT},
            {"role": "user", "content": state["raw_history"]}
        ],
        "temperature": 0.1,
        "max_tokens": 250,
        "stop": ["<|eot_id|>", "<|end_of_text|>"]
    }
    with span(tid, "planner", model="planner"):
        response = fetch_with_spinner(url, payload, "Generating secure plan...")
    plan = response['choices'][0]['message']['content'].strip()
    print(f"\n📋 [Interpreter] Control Flow Plan Captured:\n{plan}\n")

    audit.emit(tid, "planner", "plan_captured", plan=plan, step_count=state.get("step_count", 0) + 1)
    return {"blog_plan": plan, "step_count": state.get("step_count", 0) + 1}

def human_approval_gate(state: AgentState):
    tid = state.get("trace_id", "")
    audit = get_audit()

    # 2. HITL Approval Gate
    if state["blog_plan"] == "EXECUTION_HALTED":
        return {"blog_plan": state["blog_plan"]}

    print("⚠️ [HITL] Approval required to proceed to execution + formatting phase.")
    user_input = input("Approve plan? (y/N): ")
    approved = user_input.lower() == 'y'
    audit.emit(tid, "approval", "decision", approved=approved)
    if not approved:
        print("🛑 [HITL] Execution aborted by operator.")
        return {"blog_plan": "EXECUTION_HALTED"}
    return {"blog_plan": state["blog_plan"]}

def executor_agent(state: AgentState, interpreter):
    tid = state.get("trace_id", "")
    audit = get_audit()

    # Short-circuit if an upstream control halted execution.
    if state.get("blog_plan") == "EXECUTION_HALTED":
        return {"tool_output": "", "final_markdown": "Execution halted by security controls."}

    # CaMeL: the PRIVILEGED planner chose the tool call (control flow); the
    # quarantined coder never executes anything. Pull the planner's command,
    # falling back to a harmless read-only probe if none was emitted.
    match = re.search(r'COMMAND:\s*(.+)', state.get("blog_plan", ""))
    command = match.group(1).strip() if match else "uname -a"
    audit.emit(tid, "executor", "command_selected", command=command,
               source=("planner" if match else "fallback"))

    print(f"⚙️  [Executor] Detonating planner-chosen command in MicroVM: {command!r}")
    # Policy gate (forbidden-signature check) + vsock detonation in the sandbox.
    # The interpreter emits the policy/vsock spans under the same trace_id.
    tool_output = interpreter.execute_in_sandbox(command, trace_id=tid)
    print(f"📦 [Sandbox Return]:\n{tool_output}\n")

    return {
        "tool_command": command,
        "tool_output": tool_output,
        "step_count": state.get("step_count", 0) + 1,
    }

def coder_agent(state: AgentState):
    tid = state.get("trace_id", "")
    audit = get_audit()

    if state["blog_plan"] == "EXECUTION_HALTED":
        return {"final_markdown": "Execution halted by security controls."}

    print("🛠️ [Q-LLM] Qwen-1.5B (Coder) formatting sandbox output + logs...")
    url = "http://localhost:4000/v1/chat/completions"

    # Keep the control command out of the prose; the coder sees the UNTRUSTED
    # sandbox output and raw logs (CaMeL: untrusted data reaches only the Q-LLM).
    plan_for_blog = re.sub(r'(?im)^\s*COMMAND:.*$', '', state["blog_plan"]).strip()
    sandbox_output = state.get("tool_output", "")
    prompt = (
        f"Write a Markdown blog post using this structure:\n{plan_for_blog}\n\n"
        f"Verified command output captured from the sandbox:\n{sandbox_output}\n\n"
        f"Raw session logs:\n{state['raw_history']}"
    )
    payload = {
        "model": "coder",
        "messages": [
            {"role": "system", "content": CODER_SYSTEM_PROMPT},
            {"role": "user", "content": prompt}
        ],
        "temperature": 0.1,
        "max_tokens": 600,
        "stop": ["<|im_end|>", "<|endoftext|>"]
    }
    with span(tid, "coder", model="coder"):
        response = fetch_with_spinner(url, payload, "Composing markdown...")
    raw_markdown = response['choices'][0]['message']['content'].strip()

    # 3. Sanitization. NOTE: a single-pass regex, NOT a real HTML DOM sanitizer.
    # It strips markdown image tags ![alt](url) to block the URL-pixel
    # exfiltration PoC. Reference-style images, raw <img>, and autolinks are
    # NOT covered — see review notes before trusting it as defense-in-depth.
    stripped = len(re.findall(r'!\[.*?\]\(.*?\)', raw_markdown))
    safe_markdown = re.sub(r'!\[.*?\]\(.*?\)', '[IMAGE BLOCKED BY SANITIZER]', raw_markdown)
    audit.emit(tid, "coder", "sanitize", images_stripped=stripped, out_len=len(safe_markdown))

    return {"final_markdown": safe_markdown, "step_count": state.get("step_count", 0) + 1}

def build_orchestrator(interpreter):
    workflow = StateGraph(AgentState)
    workflow.add_node("planner", planner_agent)
    workflow.add_node("approval", human_approval_gate)
    workflow.add_node("executor", lambda s: executor_agent(s, interpreter))
    workflow.add_node("coder", coder_agent)

    workflow.set_entry_point("planner")
    workflow.add_edge("planner", "approval")
    workflow.add_edge("approval", "executor")
    workflow.add_edge("executor", "coder")
    workflow.add_edge("coder", END)

    return workflow.compile()
