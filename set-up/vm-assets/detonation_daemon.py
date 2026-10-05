#!/usr/bin/env python3
"""
Detonation daemon — runs INSIDE the ephemeral MicroVM.

Architecture (CaMeL / "Design-B"): the trusted control plane lives on the
macOS host. It validates a command (see camel_interpreter.py) and then hands
it to THIS daemon to execute inside the disposable sandbox. The host reaches
the daemon only over virtio-vsock, which launch_vm.go bridges to the host
loopback at 127.0.0.1:5000. The sandbox therefore needs no IP route back to
the host, so isolation is preserved: a compromised command cannot reach the
governance layer, it can only run here and return bytes.

Protocol (matches camel_interpreter.CaMeLInterpreter.execute_in_sandbox):
    POST /  { "command": "<shell command>" }  ->  200 { "output": "<stdout>" }

Stdlib only — no pip install — so the daemon is available the instant the
VM boots.
"""
import json
import socket
import subprocess

VSOCK_PORT = 5000
EXEC_TIMEOUT = 20  # a runaway command cannot hang the chamber forever


def run_command(command: str) -> str:
    try:
        proc = subprocess.run(
            command, shell=True, capture_output=True, text=True, timeout=EXEC_TIMEOUT
        )
        out = proc.stdout
        if proc.stderr:
            out += ("\n[stderr]\n" + proc.stderr)
        return out.strip()
    except subprocess.TimeoutExpired:
        return f"[detonation-daemon] command timed out after {EXEC_TIMEOUT}s"
    except Exception as exc:  # noqa: BLE001 - surface anything to the caller
        return f"[detonation-daemon] execution error: {exc}"


def read_http_request(conn) -> bytes:
    """Read one HTTP/1.1 request (headers + Content-Length body)."""
    data = b""
    while b"\r\n\r\n" not in data:
        chunk = conn.recv(4096)
        if not chunk:
            return data
        data += chunk
    head, _, body = data.partition(b"\r\n\r\n")
    length = 0
    for line in head.split(b"\r\n"):
        if line.lower().startswith(b"content-length:"):
            try:
                length = int(line.split(b":", 1)[1].strip())
            except ValueError:
                length = 0
    while len(body) < length:
        chunk = conn.recv(4096)
        if not chunk:
            break
        body += chunk
    return head + b"\r\n\r\n" + body


def handle(conn) -> None:
    try:
        raw = read_http_request(conn)
        _, _, body = raw.partition(b"\r\n\r\n")
        try:
            payload = json.loads(body.decode("utf-8", "replace") or "{}")
        except json.JSONDecodeError:
            payload = {}
        command = payload.get("command", "")
        trace_id = payload.get("trace_id", "")
        # Structured, correlated log line (shares the host request's trace_id).
        print(json.dumps({"trace_id": trace_id, "span": "vm.exec", "command": command[:500]}), flush=True)
        output = run_command(command) if command else "[detonation-daemon] no command provided"
        resp_body = json.dumps({"output": output, "trace_id": trace_id}).encode("utf-8")
        resp = (
            b"HTTP/1.1 200 OK\r\n"
            b"Content-Type: application/json\r\n"
            b"Content-Length: " + str(len(resp_body)).encode() + b"\r\n"
            b"Connection: close\r\n\r\n" + resp_body
        )
        conn.sendall(resp)
    finally:
        conn.close()


def main() -> None:
    srv = socket.socket(socket.AF_VSOCK, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.bind((socket.VMADDR_CID_ANY, VSOCK_PORT))
    srv.listen(8)
    print(f"[detonation-daemon] listening on vsock port {VSOCK_PORT}", flush=True)
    while True:
        conn, _ = srv.accept()
        handle(conn)


if __name__ == "__main__":
    main()
