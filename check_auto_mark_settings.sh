#!/bin/bash

# This script checks the current auto-mark settings on the server
# Usage: ./check_auto_mark_settings.sh [server_url]

# Default to localhost if no URL provided, otherwise use the provided URL
if [ -z "$1" ]; then
    # Try to detect if we're running in production or locally
    if ping -c 1 connect.hsannu.com &>/dev/null; then
        SERVER_URL="https://connect.hsannu.com"
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

# Check auto-mark settings
echo ""
echo "Checking auto-mark settings..."
RESPONSE=$(curl -s "$SERVER_URL/api/settings/auto-mark")
echo "$RESPONSE"

# Show Shanghai time equivalent for the configured auto-mark time
if [[ "$RESPONSE" == *"hour"* && "$RESPONSE" == *"minute"* ]]; then
    # Try to extract hour and minute values using regex
    HOUR=$(echo "$RESPONSE" | grep -o '"hour":[0-9]*' | cut -d':' -f2)
    MINUTE=$(echo "$RESPONSE" | grep -o '"minute":[0-9]*' | cut -d':' -f2)
    
    if [[ -n "$HOUR" && -n "$MINUTE" ]]; then
        # Calculate Shanghai time (UTC+8)
        SHANGHAI_HOUR=$(( (HOUR + 8) % 24 ))
        echo ""
        echo "Auto-mark time: ${HOUR}:${MINUTE} UTC = ${SHANGHAI_HOUR}:${MINUTE} Shanghai time"
    fi
fi

echo ""
echo "To update settings, use:"
echo "curl -X POST \"$SERVER_URL/api/settings/auto-mark\" \\"
echo "     -H \"Content-Type: application/json\" \\"
echo "     -d '{\"hour\": 20, \"minute\": 12, \"enabled\": true}'" 