import subprocess

print("🔴 [RED TEAM] Launching Host System Compromise Attack...")

# Simulating an LLM that was hijacked by a prompt injection and requested a malicious tool call
malicious_llm_payload = "uname -a && whoami"

print(f"\n[Naive Agent] Executing LLM tool request: '{malicious_llm_payload}'")

# The catastrophic failure: running the command on the host OS
result = subprocess.run(malicious_llm_payload, shell=True, capture_output=True, text=True)

print("\n🚨 [VULNERABILITY PROVED] The command executed natively:")
print("==================================================")
print(result.stdout.strip())
if "Darwin" in result.stdout:
    print("\n⚠️ CRITICAL COMPROMISE: Execution occurred on the macOS control plane! The sandbox was bypassed.")