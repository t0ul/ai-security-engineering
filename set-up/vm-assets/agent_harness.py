import sys
import time
import threading
import requests
import json
from typing import TypedDict
from langgraph.graph import StateGraph, END

# 1. Define the A2A State Machine
class AgentState(TypedDict):
    raw_history: str
    blog_plan: str
    final_markdown: str

# Helper Function: Fetch with CLI Spinner
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
        response.raise_for_status()
        result = response.json()
    finally:
        done = True
        t.join()
        # Clear the spinner line
        sys.stdout.write('\r' + ' ' * (len(message) + 4) + '\r')
        sys.stdout.flush()
        
    return result

# 2. Planner Agent (Llama-3.2-3B)
def planner_agent(state: AgentState):
    print("🧠 [Planner] Llama-3.2 analyzing terminal history...")
    url = "http://192.168.64.1:11435/v1/chat/completions"
    payload = {
        "messages": [
            {"role": "system", "content": "You are a concise planning assistant. Outline exactly 3 bullet points for a blog post based on terminal commands. Output ONLY the 3 bullet points."},
            {"role": "user", "content": state["raw_history"]}
        ],
        "temperature": 0.1,
        "max_tokens": 250,
        "stop": ["<|eot_id|>", "<|end_of_text|>"]
    }
    
    response = fetch_with_spinner(url, payload, "Generating outline...")
    plan = response['choices'][0]['message']['content'].strip()
    print(f"\n📋 Plan Generated:\n{plan}\n")
    
    return {"blog_plan": plan}

# 3. Tool/Coder Agent (Qwen-1.5B)
def coder_agent(state: AgentState):
    print("🛠️ [Coder] Qwen-1.5B executing formatting based on plan...")
    url = "http://192.168.64.1:11436/v1/chat/completions"
    
    # A2A Handoff: Qwen receives Llama's plan
    prompt = f"Write a Markdown blog post using this structure:\n{state['blog_plan']}\n\nInclude these raw logs:\n{state['raw_history']}"
    
    payload = {
        "messages": [
            {"role": "system", "content": "You are a technical markdown formatter. Write a brief post and finish cleanly. Do not repeat text."},
            {"role": "user", "content": prompt}
        ],
        "temperature": 0.1,
        "max_tokens": 600,
        "stop": ["<|im_end|>", "<|endoftext|>"]
    }
    
    response = fetch_with_spinner(url, payload, "Composing markdown...")
    markdown = response['choices'][0]['message']['content'].strip()
    
    return {"final_markdown": markdown}

# 4. Wire the LangGraph Orchestrator
workflow = StateGraph(AgentState)

workflow.add_node("planner", planner_agent)
workflow.add_node("coder", coder_agent)

workflow.set_entry_point("planner")
workflow.add_edge("planner", "coder")
workflow.add_edge("coder", END)

app = workflow.compile()

# 5. Execute the Loop
if __name__ == "__main__":
    dummy_zsh_history = """
    cd ~/projects/ai-security
    git status
    nmap -sV 10.0.0.5
    """
    
    print("==============================================")
    print("🔄 STARTING LANGGRAPH MULTI-AGENT LOOP")
    print("==============================================\n")
    
    initial_state = {"raw_history": dummy_zsh_history, "blog_plan": "", "final_markdown": ""}
    
    # Run the state machine
    final_state = app.invoke(initial_state)
    
    print("==============================================")
    print("✅ FINAL AGENT OUTPUT (From Qwen-1.5B)")
    print("==============================================")
    print(final_state["final_markdown"])