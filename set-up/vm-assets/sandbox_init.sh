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

echo "\n▶️  Executing Agent Harness..."
if [ -f "agent_harness.py" ]; then
    python3 agent_harness.py
fi