# Live Run Runbook — the pure-Go stack

Bring the whole agent up on the Mac and run the live checks. All commands run from
the course repo root unless noted:

```
~/Desktop/AI-Security-Engineering/ai-security-engineering
```

## Offline vs live (read first)
- **Offline** (no model, always runnable): `go test ./...` and the ADD red-team
  scorecard `go run ./cmd/scorecard`. ADD cases are white-box in-process
  attack/defense pairs — they prove the controls deterministically and **do not
  call a live model**. Green ≠ "beaten against llama/qwen".
- **Live** (needs the models serving): the eval F1 gate `cmd/livecheck` and the
  LLM extractor/ensemble, which call the gouncer gateway.
- **Step 0 — preflight:** `go run ./cmd/preflight` reports whether the GGUFs are
  downloaded and whether the ports answer, and prints exactly what to run next.
- **Live tests:** `go test -tags live ./livetest/ -v` boots the models + gateway
  in-process and runs the model-dependent checks (live eval F1, live
  prompt-injection through the real extractor). They Skip if models aren't set up.
  `go run ./cmd/livecheck` is the one-shot F1 gate (same stack).

## Topology & ports

```
 eval / controlplane ──HTTP──▶ gouncer :4000 ──┬──▶ planner  llama-3.2-3b  :11435
   (model "planner"/"coder")                   └──▶ coder    qwen-1.5b     :11436

 controlplane.Interpreter ──HTTP──▶ 127.0.0.1:5000 ──(launchvm vsock bridge)──▶ detonationd (guest vsock:5000)
```

| Component | Port | Launched by |
| --- | --- | --- |
| planner (llama.cpp) | 11435 | `cmd/modeld` |
| coder (llama.cpp) | 11436 | `cmd/modeld` |
| gouncer gateway | 4000 | fleet `cmd/gouncer` + `set-up/gouncer.json` |
| detonation daemon | vsock 5000 → 127.0.0.1:5000 | `cmd/launchvm` (in-guest `sandbox_init.sh`) |

## 0. One-time provisioning

Downloads the kernel, rootfs, and both models (verified), and cross-compiles the
in-guest daemon into `set-up/vm-assets/detonationd`:

```bash
go run ./cmd/prepareassets
```

## 1. Models — terminal 1

```bash
go run ./cmd/modeld
```
Wait for `[modelserve] all servers ready`. If it prints `… is already serving`,
a previous `modeld` still holds the ports — stop that one first.

## 2. Gateway — terminal 2

gouncer lives in the sibling fleet module, so build it once, then run it against
the route config in this repo:

```bash
( cd ../fleet/gouncer && go build -o /tmp/gouncer ./cmd/gouncer )
/tmp/gouncer set-up/gouncer.json
```
Expect `gouncer listening on :4000 (2 routes)`. Routes: `planner`→11435,
`coder`→11436 (see `set-up/gouncer.json`).

## 3. MicroVM — terminal 3

vz needs a codesigned binary with the virtualization entitlement, so build + sign
(not `go run`):

```bash
# rebuild the guest daemon first — a bare launchvm does NOT, and a stale baked
# detonationd silently drops the hardened no-shell argv path (arg-injection/ASI05).
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o set-up/vm-assets/detonationd ./cmd/detonationd
go build -o launchvm ./cmd/launchvm
codesign --entitlements set-up/entitlements.plist -s - --force launchvm
./launchvm                                   # bridge defaults to 127.0.0.1:5000
# macOS AirPlay Receiver squats on :5000. If the bridge logs "address already in
# use", either turn AirPlay Receiver off (System Settings → General → AirDrop &
# Handoff) or move the bridge and tell the app to match:
#   ./launchvm -bridge 127.0.0.1:5050   +   cmd/webapp -microvm http://127.0.0.1:5050
```
Watch for `vsock bridge up: 127.0.0.1:<port> -> guest vsock:5000` and, in the guest
console, `detonationd started`.

**Assets are already staged** (`set-up/vm-assets/`: `Image`, `alpine-root`,
`initramfs*`, `sandbox_init.sh`, a static `detonationd`, `vmfetch`, both GGUFs), so
no `prepareassets` download is needed. Still rebuild the guest `detonationd` (one
line above) whenever `cmd/detonationd` or `sandbox/` changed — the baked binary is
a gitignored artifact and can lag the source, and nothing fails loudly when it does:
the legacy `command` path keeps working while `argv` quietly returns "no command
provided".

**Verify the chamber (acceptance checks once booted).** The daemon speaks HTTP/1.1
over the bridge (any path; JSON body in, `{"output","trace_id"}` out), so you can
probe it directly without the app — the hardened `argv` form must return output,
not "no command provided":
```bash
curl -s -X POST http://127.0.0.1:5050/ \
  -d '{"argv":["sh","-c","uname -a; whoami; ip addr 2>&1 | head -3"],"trace_id":"verify"}'
```
1. **Isolation works** — that probe (or an allow-listed `sandbox_exec` of
   `uname -a && whoami` via `controlplane.Interpreter` → vsock:5000) returns output
   from *inside* the VM (`Linux localhost … aarch64`), not the host.
2. **Non-root** — that `whoami` is `sandbox` (uid 1000), not `root` (the guest drops
   privileges).
3. **Egress dead** — the probe's `ip addr` shows only `lo` (no `eth0`, no route);
   a direct `web_fetch` from the guest to a non-allow-listed host fails (no NIC);
   only a `vmfetch` through the host broker (netpolicy chokepoint) to an allow-listed
   host succeeds.
4. **Ephemeral** — state does not persist across a reboot (Alpine in RAM).
5. `ForbiddenSignatures` / argv-allowlist still refuse a destructive command
   (`AllowlistBypass` holds live, not just in the offline scorecard).

## 4. Live checks

**Eval F1 gate (closes the 1.txt ≥0.87 target).** Default `GATEWAY_URL` is
`:4000`, so the extractor's LLM path flows through gouncer → planner:

```bash
EXTRACT_MODE=llm go run ./cmd/eval --tool testdata/emaildrop/labels/1.json
EXTRACT_MODE=llm go run ./cmd/eval --tool testdata/emaildrop/labels/3.json   # expect F1 1.00
```

**Control-plane CaMeL loop** (planner → HITL → executor → MicroVM → coder; exfil
sanitizer). It posts commands to the real MicroVM over the vsock bridge:

```bash
GATEWAY_URL=http://127.0.0.1:4000/v1/chat/completions \
MICROVM_URL=http://127.0.0.1:5000 \
go run ./cmd/controlplane
```
`MICROVM_URL` must match the `launchvm -bridge` address — use
`http://127.0.0.1:5050` if you moved the bridge off AirPlay's :5000.
Approve the plan at the `Approve plan? (y/N)` prompt; confirm the final output has
the image URL stripped and `chain_ok=true`.

## 5. Teardown

Ctrl-C each terminal (3 → 2 → 1). `modeld` and `launchvm` both shut their
children/VM down cleanly on SIGINT.

## Notes

- `modeld` health-gates on llama.cpp `/health`; it refuses to start over a port
  that is already serving and aborts if a server exits during startup.
- Keep model servers bound to `127.0.0.1` (least exposure); only gouncer is the
  intended ingress, and even it is loopback here.
- The guest runs the static Go `detonationd` (no python). Rebuild it via
  `go run ./cmd/prepareassets` after changing `cmd/detonationd` or the `sandbox/`
  package.
