#!/bin/bash

# This simple script forces auto-marking on the server immediately
# Usage: ./force_auto_mark.sh [server_url]

# Default to localhost if no URL provided, otherwise use the provided URL
if [ -z "$1" ]; then
    # Try to detect if we're running in production or locally
    if ping -c 1 connect.hsannu.com &>/dev/null; then
        SERVER_URL="http://connect.hsannu.com"
        echo "No server URL provided, using production server: $SERVER_URL"
    else
        SERVER_URL="http://localhost:2000"
        echo "No server URL provided, using local server: $SERVER_URL"
    fi
else
    SERVER_URL="$1"
    echo "Using provided server URL: $SERVER_URL"
fi

# Current UTC time for reference
echo "Current UTC time: $(date -u)"

# First try the direct test endpoint (simplest)
echo ""
echo "Method 1: Using /test/auto-mark endpoint..."
curl -s "$SERVER_URL/api/test/auto-mark"
echo ""

# Then try forcing via the settings endpoint (most reliable)
echo ""
echo "Method 2: Using force flag with settings endpoint..."
curl -s -X POST "$SERVER_URL/api/settings/auto-mark?run_now=true" \
     -H "Content-Type: application/json" \
     -d "{}"
echo ""

echo ""
echo "Auto-marking force requests sent!"
echo "Check server logs for [AUTO-MARK] entries to confirm operation." 