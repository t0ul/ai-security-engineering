# MicroVM hardening plan — "use it as it should've been"

Durable plan (written before a context compaction). Goal: make the Apple `vz`
MicroVM a **genuine detonation chamber** and the **real execution substrate for
executing agent tools**, not just a demo. Grounded in a code-verified review.

## Status (2026-10-06) — all slices landed; full suite green on the Mac
- **P1 egress-deny** ✅ `cmd/launchvm` attaches no network device; `bringUpCmd`
  drops `udhcpc`/`ip link`/`apk`/resolv.conf; `sandbox_init.sh` drops the NTP
  sync. Guest has no interface/route/DNS.
- **P4 allowlist** ✅ `controlplane.Policy` (argv[0] allowlist + `argcheck` on
  args + blocklist tripwire) is the primary gate; `ForbiddenSignatures` demoted
  to secondary. ADD `redteam.AllowlistBypass` 100%→0%.
- **P2 route executing tool through the VM** ✅ `controlplane.SandboxExecTool`
  (`sandbox_exec`) runs argv in the VM via `Interpreter.ExecuteArgv`; wired
  through the gustoms gateway in `cmd/mcpdemo`. Daemon speaks `{argv}` (no shell)
  with legacy `{command}` kept for host dev. (web_fetch stays host-side behind
  netpolicy by design — the chamber has no egress.)
- **P3 ephemeral** ✅ `sandbox.Daemon.capture` runs each detonation in a fresh
  temp dir removed afterwards.
- **P5 non-root** ✅ `sandbox_init.sh` runs `detonationd` as user `sandbox`.
- **P6 docs** ✅ syllabus, BACKLOG, this plan truthed up.
- **P7 egress broker** ✅ (supersedes "web_fetch stays host-side"): executing
  tools now run IN the VM and reach the network only through a host-side
  netpolicy broker over vsock — the guest still has no NIC. New `broker` package
  (host tunnel + guest `HTTPClient`), `sandbox.DialHost` (guest→host vsock),
  `cmd/vmfetch` (in-VM fetch tool, cross-built by prepareassets, installed on
  guest PATH by sandbox_init.sh), `controlplane.ExecuteFetch` + `SandboxFetchTool`
  (web_fetch that detonates `vmfetch` in the VM), and `launchvm` host wiring
  (`-allow` egress allowlist, vsock:5001 listener → broker). Host-tested over TCP
  (`broker` tests, `cmd/mcpdemo` §11–12); netpolicy remains the single egress
  authority, now enforced at the broker.

**Remaining = t's step only:** re-boot the VM (`launchvm -allow <hosts>` +
codesign) and confirm live: no-egress `curl` (direct) fails, `uname` works, `id`
shows non-root, and `sandbox_exec ["vmfetch","https://<allowed>"]` succeeds while
a non-allowlisted host is refused at the broker. No host-side Go work left.

## Rhythm (unchanged)
Claude writes Go into the connected folders. **t runs `go test`, all `git`, and
anything that boots the VM (`launchvm`) on the Mac.** `go 1.27.1`, `go mod tidy`,
commits authored as `t <toul@hey.com>`, zero Claude attribution. Fleet imported
via `replace => ../fleet/*`.

## Verified current state (facts, not claims)
- MicroVM used ONLY by `cmd/controlplane` → `controlplane.Orchestrator` →
  `controlplane.Interpreter.ExecuteInSandbox` → vsock → `cmd/detonationd`.
- Email→calendar agent (`pipeline`/`watcher`/`webapp`/`emaildrop`) never touches
  the VM; extraction is single-LLM, host-run, defended by `agent/guard` +
  spotlighting + output sanitizer. (That's correct for a parse-only tool.)
- `cmd/launchvm` attaches `vz.NewNATNetworkDeviceAttachment()` and bring-up runs
  `udhcpc` → **guest has full outbound internet** (the big hole).
- One persistent guest/chroot reused across detonations (no per-run reset).
- Policy = `controlplane.ForbiddenSignatures` **blocklist** (bypassable).
- Guest commands run as **root**, no seccomp / cap-drop.
- `web_fetch` (MCP, the one executing tool + SSRF surface) runs on the HOST.

## Target architecture
Executing/untrusted-code tools (browse/shell/RCE labs) run **inside an
egress-denied, ephemeral MicroVM**, reached over vsock, with **structured args**
(argcheck) not shell strings, behind an **allowlist** (not a blocklist), as
**non-root**. Non-executing tools (email extraction) stay on the host, guarded.
Any network a tool needs goes out **only through the host netpolicy proxy**
(allow-list + no-IMDS + dial-pinned), never the guest's own stack.

## Work items (ordered; each is a small, testable slice)

### P1 — Egress-deny the chamber (HIGH, do first; safe post-Python)
Since Python is gone, the guest needs no `apk`/internet.
- `cmd/launchvm/main.go`: remove `NewNATNetworkDeviceAttachment()` +
  `SetNetworkDevicesVirtualMachineConfiguration`. Strip `udhcpc`/`ip link`/`apk`
  from `bringUpCmd`; keep: mount virtiofs assets, chroot Alpine, run
  `/mnt/assets/detonationd`.
- Acceptance: VM boots with NO default route; `detonationd` serves on vsock;
  `ExecuteInSandbox("curl -m2 https://example.com")` fails (no egress) while
  `ExecuteInSandbox("uname -a")` works. (t verifies on Mac with `launchvm`.)

### P2 — Route an executing tool THROUGH the VM
- New `controlplane` (or `sandboxtool`) type `SandboxTool` implementing
  `tool.Tool`: validates args with `argcheck` (schema, no shell metas, confined
  paths), builds an **arg-array** command, and runs it via
  `Interpreter.ExecuteInSandbox`. No shell-string interpolation.
- Re-implement `web_fetch` as a sandbox tool: the fetch executes in the VM
  (`detonationd` runs it), but the URL is first cleared by host `netpolicy`
  (allow-list + dial-pin) OR the fetch is performed host-side by the netpolicy
  client and only *parsing* runs in the VM — decide per threat model (prefer:
  netpolicy-guarded fetch on host, risky parsing/execution in VM).
- Acceptance: a unit test (host side) proving arg validation + that a shell-meta
  arg is rejected before detonation; an integration note for the live VM path.

### P3 — Ephemeral per-detonation isolation
- Option A (cheapest): per-detonation fresh working dir + process in the guest
  (detonationd runs each command in a new temp cwd, cleaned after).
- Option B (strongest): reset/reboot the VM (or a fresh microVM) per detonation
  — heavier; design + measure boot time (~1s claim).
- Acceptance: command A writing `/tmp/x` is NOT visible to command B.

### P4 — Allowlist policy, drop the blocklist
- Replace `ForbiddenSignatures` with an **allowlist** of permitted argv[0]
  (e.g. `uname,ls,cat,tail,git`) + argcheck on args. Keep the blocklist only as a
  secondary tripwire. Update the `controlplane` policy gate + its ADD case.
- Acceptance: `rm --recursive /` and `base64`-obfuscated payloads are refused by
  the allowlist (the blocklist missed these); ASR case added.

### P5 — Least privilege in the guest (defense-in-depth)
- Run detonations as a **non-root** user in the chroot; drop capabilities;
  (stretch) seccomp profile. Adjust `sandbox_init.sh` / bring-up.
- Acceptance: `id` inside a detonation shows non-root; still functional.

### P6 — Docs truth-up
- `scratch/syllabus.md` + `docs/BACKLOG.md`: reflect egress-deny, sandbox tool
  routing, allowlist, ephemerality. (Syllabus MicroVM paragraph already corrected
  to the honest current state — update again when P1–P5 land.)

## Notes / gotchas
- `detonationd` is Linux/arm64 static (CGO off); cross-built into `vm-assets/` by
  `cmd/prepareassets`. `launchvm` needs codesign:
  `go build -o launchvm ./cmd/launchvm && codesign --entitlements set-up/entitlements.plist -s - --force launchvm && ./launchvm`.
- Anything requiring a booted VM is **t's** step; Claude ships the Go + unit
  tests and a one-line "verify on Mac" for each slice.
- Keep netpolicy as the single egress authority; never give the guest its own
  internet again.

## Suggested order to execute next session
P1 (egress-deny) → P4 (allowlist) → P2 (sandbox tool + web_fetch) → P3
(ephemeral) → P5 (non-root) → P6 (docs). P1+P4 are pure Go + small; P2 is the
one that makes the VM central to the agent.
