import requests

class CaMeLInterpreter:
    def __init__(self):
        print("⚖️  Initializing CaMeL Policy Interpreter...")
        # Assuming the MicroVM is bridged to localhost:5000 via the virtualization framework
        self.microvm_url = "http://localhost:5000" 
    
    def execute_in_sandbox(self, command: str) -> str:
        """
        Outbound Policy Engine: Validates the command, then pushes it across 
        the network boundary to the MicroVM for safe detonation.
        """
        forbidden_commands = ["rm -rf", "nc -e", "mkfifo", "> /dev/tcp"]
        for forbidden in forbidden_commands:
            if forbidden in command:
                return f"🚫 [Interpreter] BLOCKED: Malicious signature '{forbidden}' detected."
                
        print(f"✅ [Interpreter] Policy passed. Pushing '{command}' to MicroVM...")
        
        try:
            response = requests.post(self.microvm_url, json={"command": command}, timeout=15)
            response.raise_for_status()
            raw_output = response.json().get("output", "")
            return raw_output
        except requests.exceptions.RequestException as e:
            return f"❌ [Interpreter] MicroVM connection failed: {e}"