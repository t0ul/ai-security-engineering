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

chmod +x /mnt/assets/detonationd
# Dedicated unprivileged user; ignore the error if it already exists.
adduser -D sandbox 2>/dev/null || true

echo "Starting detonation daemon as non-root user 'sandbox' (vsock:5000)..."
su sandbox -c '/mnt/assets/detonationd' > /tmp/detonation.log 2>&1 &
echo "  detonationd started (PID $!) — tail /tmp/detonation.log to watch"

echo ""
echo "Interactive shell ready."
exec /bin/sh
