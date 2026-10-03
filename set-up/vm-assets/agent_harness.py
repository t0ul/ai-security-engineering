import os
import platform
import requests
from datetime import datetime

print("==============================================")
print("🛡️️  MICROVM PYTHON SECURITY ISOLATION TESTS")
print("==============================================")

# Test 1: Kernel Identity Verification
os_type = platform.system()
if os_type == "Linux":
    print(f"✔ PASS: Environment is {os_type} (Not macOS)")
else:
    print(f"❌ FAIL: Environment identifies as {os_type}")

# Test 2: Host User-Space Isolation
if os.path.exists("/Users/"):
    print("❌ FAIL: VM breached isolation and can read Mac host directories!")
else:
    print("✔ PASS: Host filesystem is completely invisible.")

# Test 3: Credential Isolation
ssh_path = os.path.expanduser("~/.ssh")
if os.path.exists(ssh_path) and os.listdir(ssh_path):
    print("❌ FAIL: Host SSH keys leaked into sandbox.")
else:
    print("✔ PASS: No host credentials detected in execution space.")

print("\n==============================================")
print("🧠 AUTONOMOUS AGENT LLM INVOCATION (BLOG WORKFLOW)")
print("==============================================")

# Dummy terminal history simulating ~/.zsh_history extraction
dummy_zsh_history = """
cd ~/projects/ai-security
git status
nmap -sV 10.0.0.5
curl -I https://huggingface.co
python3 launch_vm.py
"""

# Routing to Llama-3.2-3B (Planner Agent) on port 11435
url = "http://192.168.64.1:11435/v1/chat/completions"

payload = {
    "messages": [
        {
            "role": "system",
            "content": "You are a technical blogging assistant. Summarize the provided terminal command history into a short, structured Markdown blog post. Include a title, a brief summary of the activity, and format the raw commands within a markdown code block."
        },
        {
            "role": "user",
            "content": f"Here is the recent command history:\n{dummy_zsh_history}"
        }
    ],
    "temperature": 0.2
}

print(f"Sending reasoning request to Host Control Plane ({url})...")

try:
    # 1. Fetch LLM response via NAT Gateway
    response = requests.post(url, json=payload, timeout=30)
    response.raise_for_status()
    data = response.json()
    
    # 2. Extract the blog content
    blog_content = data['choices'][0]['message']['content'].strip()
    
    print("\n🔥 LLM Generated Blog Post:")
    print(blog_content)
    
    # 3. Write structured markdown blog entry inside the Sandbox
    timestamp = datetime.now().strftime('%Y-%m-%d')
    blog_filename = f"blog_entry_{timestamp}.md"
    
    with open(blog_filename, "w") as f:
        f.write(blog_content)
        
    print(f"\n▶️  Successfully wrote structured markdown to {blog_filename} inside Sandbox.")
        
except Exception as e:
    print(f"\n❌ Execution Failed: {e}")