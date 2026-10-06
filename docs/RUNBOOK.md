# Live Run Runbook — the pure-Go stack

Bring the whole agent up on the Mac and run the live checks. All commands run from
the course repo root unless noted:

```
~/Desktop/AI-Security-Engineering/ai-security-engineering
```

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
go build -o launchvm ./cmd/launchvm
codesign --entitlements set-up/entitlements.plist -s - --force launchvm
./launchvm
```
Watch for `vsock bridge up: 127.0.0.1:5000 -> guest vsock:5000` and, in the guest
console, `detonationd started`.

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
