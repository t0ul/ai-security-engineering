import os
import requests
from telemetry import get_audit

class CaMeLInterpreter:
    def __init__(self):
        print("⚖️  Initializing CaMeL Policy Interpreter...")
        # The MicroVM detonation daemon is reached over vsock, bridged to the host
        # loopback by launch_vm.go. Use 127.0.0.1 (not "localhost") to avoid an
        # IPv6 ::1 first-attempt when the bridge binds IPv4 only.
        self.microvm_url = os.environ.get("MICROVM_URL", "http://127.0.0.1:5000")

    def execute_in_sandbox(self, command: str, trace_id: str = "") -> str:
        """
        Outbound Policy Engine: validates the command, then pushes it across the
        vsock boundary to the MicroVM for isolated detonation. Every decision is
        recorded under the request's trace_id (policy gate + vsock result).
        """
        audit = get_audit()
        forbidden_commands = ["rm -rf", "nc -e", "mkfifo", "> /dev/tcp"]
        for forbidden in forbidden_commands:
            if forbidden in command:
                audit.emit(trace_id, "policy", "gate", decision="block",
                           signature=forbidden, command=command)
                return f"🚫 [Interpreter] BLOCKED: Malicious signature '{forbidden}' detected."

        audit.emit(trace_id, "policy", "gate", decision="allow", command=command)
        print(f"✅ [Interpreter] Policy passed. Pushing '{command}' to MicroVM...")

        try:
            response = requests.post(
                self.microvm_url,
                json={"command": command, "trace_id": trace_id},
                timeout=15,
            )
            response.raise_for_status()
            data = response.json()
            raw_output = data.get("output", "")
            audit.emit(trace_id, "vsock", "detonate", ok=True,
                       out_len=len(raw_output), vm_trace=data.get("trace_id", ""))
            return raw_output
        except requests.exceptions.RequestException as e:
            audit.emit(trace_id, "vsock", "detonate", ok=False, error=str(e))
            return f"❌ [Interpreter] MicroVM connection failed: {e}"
