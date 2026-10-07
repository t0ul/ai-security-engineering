#!/bin/sh
# In-guest sandbox init, run inside the Alpine chroot by launchvm.
#
# The chamber is egress-denied: launchvm attaches no network device, so there is
# no interface to bring up, no DNS, and no NTP (the old `ntpd ... pool.ntp.org`
# sync is gone — it would only hang against a dead network). Pure Go fold-in: no
# pip, no python. The detonation daemon is a static binary served from the shared
# /mnt/assets mount and reached only over vsock:5000.
#
# Least privilege (M6): the daemon runs as a non-root user, so every command it
# detonates inherits a non-root, no-network context. Each detonation also runs in
# a fresh temp dir that is removed afterwards (see sandbox.Daemon), so one
# detonation cannot observe another's files.
echo "=============================================="
echo "  INITIALIZING EPHEMERAL SANDBOX (no egress)"
echo "=============================================="

if [ ! -f /mnt/assets/detonationd ]; then
    echo "  WARNING: /mnt/assets/detonationd not found."
    echo "  Build it first:  go run ./cmd/prepareassets   (cross-compiles it into vm-assets/)"
    echo ""
    echo "Interactive shell ready."
    exec /bin/sh
fi

# The assets live on a read-only virtiofs mount, so chmod is refused there — the
# exec bit is already carried over from the host build. Best-effort, never fatal.
chmod +x /mnt/assets/detonationd 2>/dev/null || true

# Put the in-VM fetch tool on PATH so sandbox_exec can run it by name as
# `vmfetch <url>` (detonationd exec's argv[0] via PATH lookup, and /mnt/assets is
# not on PATH). It reaches the net only through the host egress broker over vsock.
mkdir -p /usr/local/bin
if [ -f /mnt/assets/vmfetch ]; then
    cp /mnt/assets/vmfetch /usr/local/bin/vmfetch && chmod 0755 /usr/local/bin/vmfetch
    echo "  vmfetch installed to /usr/local/bin"
else
    echo "  NOTE: /mnt/assets/vmfetch not found — re-run 'go run ./cmd/prepareassets'"
fi
# Dedicated unprivileged user; ignore the error if it already exists.
adduser -D sandbox 2>/dev/null || true

echo "Starting detonation daemon as non-root user 'sandbox' (vsock:5000)..."
su sandbox -c '/mnt/assets/detonationd' > /tmp/detonation.log 2>&1 &
echo "  detonationd started (PID $!) — tail /tmp/detonation.log to watch"

echo ""
echo "Interactive shell ready."
exec /bin/sh
