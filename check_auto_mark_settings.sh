#!/bin/bash

# Get the current UTC time
UTC_TIME=$(date -u +"%H:%M:%S")
UTC_DATE=$(date -u +"%Y-%m-%d")

# Get the current Shanghai time (UTC+8)
SHANGHAI_TIME=$(TZ="Asia/Shanghai" date +"%H:%M:%S")
SHANGHAI_DATE=$(TZ="Asia/Shanghai" date +"%Y-%m-%d")

echo "=========================================="
echo "📊 Auto-Mark Settings Checker"
echo "=========================================="
echo "Current UTC time:      $UTC_TIME ($UTC_DATE)"
echo "Current Shanghai time: $SHANGHAI_TIME ($SHANGHAI_DATE)"
echo "=========================================="

# Make API request to check auto-mark settings
RESPONSE=$(curl -s https://connect.hsannu.com/api/settings/auto-mark)

# Parse the JSON response
HOUR=$(echo $RESPONSE | grep -o '"hour":[0-9]*' | cut -d ':' -f 2)
MINUTE=$(echo $RESPONSE | grep -o '"minute":[0-9]*' | cut -d ':' -f 2)
ENABLED=$(echo $RESPONSE | grep -o '"enabled":\(true\|false\)' | cut -d ':' -f 2)

# Calculate Shanghai time (UTC+8)
SHANGHAI_HOUR=$(( (HOUR + 8) % 24 ))

echo "Auto-Mark Configuration:"
echo "- UTC Time:       $HOUR:$MINUTE"
echo "- Shanghai Time:  $SHANGHAI_HOUR:$MINUTE"
echo "- Enabled:        $ENABLED"
echo "=========================================="
echo "Note: Auto-marking should be set to 23:40 UTC (7:40 Shanghai time)"
echo "      to mark students as late at 7:40 AM China time."
echo "=========================================="
echo "To update settings, use: ./force_auto_mark.sh"
echo "To test auto-marking now: ./test_auto_mark_now.sh"
echo "==========================================" 