#!/bin/sh
# In-guest sandbox init, run inside the Alpine chroot by launchvm. Pure Go
# fold-in: no pip, no python — the detonation daemon is a static binary served
# from the shared /mnt/assets mount. Ends by exec'ing an interactive shell so the
# backgrounded daemon's parent persists for the life of the session.
echo "=============================================="
echo "  INITIALIZING EPHEMERAL SANDBOX"
echo "=============================================="

echo "Syncing system clock (NTP)..."
ntpd -d -q -n -p pool.ntp.org

echo "Starting detonation daemon (vsock:5000)..."
if [ -f /mnt/assets/detonationd ]; then
    chmod +x /mnt/assets/detonationd
    /mnt/assets/detonationd > /tmp/detonation.log 2>&1 &
    echo "  detonationd started (PID $!) — tail /tmp/detonation.log to watch"
else
    echo "  WARNING: /mnt/assets/detonationd not found."
    echo "  Build it first:  go run ./cmd/prepareassets   (cross-compiles it into vm-assets/)"
fi

echo ""
echo "Interactive shell ready."
exec /bin/sh
