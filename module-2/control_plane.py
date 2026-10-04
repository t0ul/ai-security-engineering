import sys
import time
import threading
import requests
from typing import TypedDict
from langgraph.graph import StateGraph, END
from presidio_analyzer import AnalyzerEngine
from presidio_anonymizer import AnonymizerEngine

print("==============================================")
print("🛡️  INITIALIZING HOST CONTROL PLANE")
print("==============================================")

# 1. Initialize Presidio Middleware
analyzer = AnalyzerEngine()
anonymizer = AnonymizerEngine()

def sanitize_input(raw_text: str) -> str:
    results = analyzer.analyze(text=raw_text, entities=["IP_ADDRESS", "EMAIL_ADDRESS", "SECRET_KEY"], language='en')
    sanitized = anonymizer.anonymize(text=raw_text, analyzer_results=results)
    return sanitized.text

# 2. Define LangGraph State
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
            print(f"\n❌ 400 Bad Request from {url}")
            print(f"Server Error Log: {response.text}\n")
            
        response.raise_for_status()
        result = response.json()
    finally:
        done = True
        t.join()
        sys.stdout.write('\r' + ' ' * (len(message) + 4) + '\r')
        sys.stdout.flush()
    return result

# 3. Planner Agent (Llama-3.2-3B)
def planner_agent(state: AgentState):
    print("🧠 [Planner] Llama-3.2 analyzing sanitized history...")
    url = "http://localhost:11435/v1/chat/completions"
    payload = {
        "model": "local",
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

# 4. Tool/Coder Agent (Qwen-1.5B)
def coder_agent(state: AgentState):
    print("🛠️ [Coder] Qwen-1.5B composing formatted Markdown post...")
    url = "http://localhost:11436/v1/chat/completions"
    prompt = f"Write a Markdown blog post using this structure:\n{state['blog_plan']}\n\nInclude these raw logs:\n{state['raw_history']}"
    payload = {
        "model": "local",
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

# 5. Wire the LangGraph Orchestrator
workflow = StateGraph(AgentState)
workflow.add_node("planner", planner_agent)
workflow.add_node("coder", coder_agent)
workflow.set_entry_point("planner")
workflow.add_edge("planner", "coder")
workflow.add_edge("coder", END)
app = workflow.compile()

if __name__ == "__main__":
    dummy_zsh_history = """
    cd ~/projects/ai-security
    git status
    nmap -sV 10.0.0.5
    curl -I https://huggingface.co
    """
    
    print("\n[Middleware] Intercepting and scrubbing untrusted inputs...")
    safe_history = sanitize_input(dummy_zsh_history)
    print(f"Scrubbed Payload:\n{safe_history}\n")
    
    print("🔄 STARTING LANGGRAPH MULTI-AGENT LOOP")
    initial_state = {"raw_history": safe_history, "blog_plan": "", "final_markdown": ""}
    final_state = app.invoke(initial_state)
    
    print("==============================================")
    print("✅ FINAL AGENT OUTPUT (From Qwen-1.5B)")
    print("==============================================")
    print(final_state["final_markdown"])
    print("\n[Control Plane] Ready to push output to MicroVM execution sandbox.")