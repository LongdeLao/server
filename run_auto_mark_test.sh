#!/bin/bash

echo "=========================================="
echo "🚀 Auto-Mark Quick Test Runner"
echo "=========================================="

# Get the current UTC time
UTC_TIME=$(date -u +"%H:%M:%S")
UTC_DATE=$(date -u +"%Y-%m-%d")

# Get the current Shanghai time (UTC+8)
SHANGHAI_TIME=$(TZ="Asia/Shanghai" date +"%H:%M:%S")
SHANGHAI_DATE=$(TZ="Asia/Shanghai" date +"%Y-%m-%d")

echo "Current UTC time:      $UTC_TIME ($UTC_DATE)"
echo "Current Shanghai time: $SHANGHAI_TIME ($SHANGHAI_DATE)"
echo "=========================================="

echo "Running auto-mark test with current time..."
RESPONSE=$(curl -s "https://connect.hsannu.com/api/test/auto-mark")

echo "Response from server:"
echo "$RESPONSE"
echo ""
echo "=========================================="
echo "✅ Test completed!"
echo "For more test options, run: ./test_auto_mark_now.sh"
echo "To check settings, run: ./check_auto_mark_settings.sh"
echo "To update settings, run: ./force_auto_mark.sh"
echo "==========================================" 