#!/bin/sh
echo "=============================================="
echo "⚙️  INITIALIZING EPHEMERAL SANDBOX"
echo "=============================================="

echo "⏳ Syncing system clock (NTP)..."
# The -d -q -n flags force Alpine's ntpd to run in the foreground, print debug info, step the clock, and exit
ntpd -d -q -n -p pool.ntp.org

echo "📦 Installing Python dependencies..."
cd /mnt/assets
# Fails gracefully if requirements.txt doesn't exist yet
if [ -f "requirements.txt" ]; then
    pip install -r requirements.txt --break-system-packages
fi

echo "\n▶️  Starting detonation daemon (vsock:5000)..."
# Stdlib-only, so no pip needed. Background it so boot continues to the
# interactive shell. Logs stream to /tmp/detonation.log.
if [ -f "detonation_daemon.py" ]; then
    python3 /mnt/assets/detonation_daemon.py > /tmp/detonation.log 2>&1 &
    echo "   detonation daemon started (PID $!) — tail /tmp/detonation.log to watch"
else
    echo "   ⚠️  detonation_daemon.py not found in /mnt/assets"
fi