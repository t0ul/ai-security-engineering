import sys
import time
import threading
import requests
from typing import TypedDict
from langgraph.graph import StateGraph, END
from prompts import PLANNER_SYSTEM_PROMPT, CODER_SYSTEM_PROMPT

class AgentState(TypedDict):
    raw_history: str
    blog_plan: str
    final_markdown: str

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
    response = fetch_with_spinner(url, payload, "Generating secure plan...")
    plan = response['choices'][0]['message']['content'].strip()
    print(f"\n📋 [Interpreter] Control Flow Plan Captured:\n{plan}\n")
    return {"blog_plan": plan}

def coder_agent(state: AgentState):
    print("🛠️ [Q-LLM] Qwen-1.5B (Coder) extracting & formatting data...")
    url = "http://localhost:4000/v1/chat/completions"
    prompt = f"Write a Markdown blog post using this structure:\n{state['blog_plan']}\n\nInclude these raw logs:\n{state['raw_history']}"
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
    response = fetch_with_spinner(url, payload, "Composing markdown...")
    markdown = response['choices'][0]['message']['content'].strip()
    return {"final_markdown": markdown}

def build_orchestrator():
    workflow = StateGraph(AgentState)
    workflow.add_node("planner", planner_agent)
    workflow.add_node("coder", coder_agent)
    workflow.set_entry_point("planner")
    workflow.add_edge("planner", "coder")
    workflow.add_edge("coder", END)
    return workflow.compile()