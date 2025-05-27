#!/bin/bash

# This script calls the API to trigger auto-marking right now
# Usage: ./test_auto_mark_now.sh [server_url]

# Default server URL if not provided
SERVER_URL=${1:-"https://connect.hsannu.com"}

echo "Testing auto-marking with server at: $SERVER_URL"

# Get current time for log
CURRENT_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
echo "Current UTC time: $CURRENT_TIME"

# Function to make API call and display the response
call_api() {
    local url=$1
    local description=$2
    local curl_options=$3
    
    echo "---------------------------------------------"
    echo "$description"
    echo "URL: $url"
    if [ -n "$curl_options" ]; then
        echo "Options: $curl_options"
    fi
    echo "---------------------------------------------"
    
    # Make the API call and save the response
    if [ -n "$curl_options" ]; then
        # Use eval to properly handle quoted options
        RESPONSE=$(eval "curl -s $curl_options \"$url\"")
    else
        RESPONSE=$(curl -s "$url")
    fi
    STATUS=$?
    
    # Check if curl command succeeded
    if [ $STATUS -ne 0 ]; then
        echo "ERROR: Failed to connect to the server (curl exit code: $STATUS)"
        return
    fi
    
    # Display the raw response
    echo "Raw response:"
    echo "$RESPONSE"
    echo
    
    # Try to parse as JSON (but don't fail if it's not JSON)
    if command -v jq &> /dev/null; then
        echo "JSON parsed response (if valid JSON):"
        echo "$RESPONSE" | jq '.' 2>/dev/null || echo "Not valid JSON"
    fi
    echo "---------------------------------------------"
    echo
}

# First, get the current configuration
call_api "$SERVER_URL/api/settings/auto-mark" "Getting current auto-mark configuration"

# Trigger auto-marking immediately
call_api "$SERVER_URL/api/test/auto-mark" "Triggering auto-marking now"

# Also try forcing via the settings endpoint
echo "Do you want to try forcing auto-marking via settings endpoint? (y/n)"
read -r response
if [[ "$response" =~ ^([yY][eE][sS]|[yY])$ ]]; then
    call_api "$SERVER_URL/api/settings/auto-mark?run_now=true" "Forcing auto-marking via settings endpoint (POST)" "-X POST -H \"Content-Type: application/json\" -d '{}'"
fi

echo ""
echo "Auto-marking test complete! Check the server logs for detailed output."
echo "Look for log lines starting with [AUTO-MARK] or AUTO-MARK to track the process."
echo "" 