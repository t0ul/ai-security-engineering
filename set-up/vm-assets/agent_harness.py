import os
import platform
import subprocess
import requests

# need to run: ntpd -q -p pool.ntp.org to sync system clock from year 2022 to year 2026

print("==============================================")
print("🛡️  MICROVM PYTHON SECURITY ISOLATION TESTS")
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
print("🧠 AUTONOMOUS AGENT LLM INVOCATION")
print("==============================================")

url = "http://192.168.64.1:11434/v1/chat/completions"
payload = {
    "messages": [
        {
            "role": "system",
            "content": "You are a code execution agent. Output strictly raw terminal commands. Do NOT use markdown code blocks. Do NOT use backticks."
        },
        {
            "role": "user",
            "content": "Write a bash command to echo 'Hello from the Python isolated AI agent!' and print the current date."
        }
    ],
    "temperature": 0.1
}

print(f"Sending reasoning request to Host Control Plane ({url})...")

try:
    # 1. Fetch LLM response via NAT Gateway
    response = requests.post(url, json=payload, timeout=15)
    response.raise_for_status()
    data = response.json()
    
    # 2. Extract strictly the text content
    command = data['choices'][0]['message']['content'].strip()
    
    # 3. Clean markdown if the SLM hallucinates formatting
    command = command.replace("```bash", "").replace("```sh", "").replace("```", "").strip()
    
    print("\n🔥 LLM Generated Command:")
    print(command)
    print("\n▶️  Executing untrusted command inside Sandbox...")
    
    # 4. Execute safely in the Linux sandbox
    result = subprocess.run(command, shell=True, capture_output=True, text=True)
    
    print("\n--- Output ---")
    print(result.stdout)
    
    if result.stderr:
        print("--- Errors ---")
        print(result.stderr)
        
except Exception as e:
    print(f"\n❌ Execution Failed: {e}")