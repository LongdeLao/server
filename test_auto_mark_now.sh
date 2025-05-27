#!/bin/bash

# Get the current UTC time
UTC_TIME=$(date -u +"%H:%M")
UTC_DATE=$(date -u +"%Y-%m-%d")

# Get the current Shanghai time (UTC+8)
SHANGHAI_TIME=$(TZ="Asia/Shanghai" date +"%H:%M")
SHANGHAI_DATE=$(TZ="Asia/Shanghai" date +"%Y-%m-%d")

echo "=========================================="
echo "🧪 Auto-Mark Test Runner"
echo "=========================================="
echo "Current UTC time:      $UTC_TIME ($UTC_DATE)"
echo "Current Shanghai time: $SHANGHAI_TIME ($SHANGHAI_DATE)"
echo "=========================================="

# Options for the user
echo "Select a test option:"
echo "1) Run auto-marking with current time"
echo "2) Run auto-marking with custom time"
echo "3) Force auto-marking via settings API"
echo "4) Test marking a specific student's arrival"
echo ""

read -p "Enter option (1-4): " OPTION

case $OPTION in
    1)
        echo "Running auto-marking with current time..."
        RESPONSE=$(curl -s "https://connect.hsannu.com/api/test/auto-mark")
        echo "Response:"
        echo "$RESPONSE"
        ;;
        
    2)
        read -p "Enter hour (0-23): " HOUR
        read -p "Enter minute (0-59): " MINUTE
        
        # Validate inputs
        if [[ ! "$HOUR" =~ ^[0-9]+$ ]] || [ "$HOUR" -lt 0 ] || [ "$HOUR" -gt 23 ]; then
            echo "Invalid hour. Must be between 0-23."
            exit 1
        fi
        
        if [[ ! "$MINUTE" =~ ^[0-9]+$ ]] || [ "$MINUTE" -lt 0 ] || [ "$MINUTE" -gt 59 ]; then
            echo "Invalid minute. Must be between 0-59."
            exit 1
        fi
        
        # Calculate Shanghai time
        SHANGHAI_HOUR=$(( (HOUR + 8) % 24 ))
        
        echo "Testing with time: $HOUR:$MINUTE UTC ($SHANGHAI_HOUR:$MINUTE Shanghai)"
        RESPONSE=$(curl -s "https://connect.hsannu.com/api/test/auto-mark?time=$HOUR:$MINUTE")
        echo "Response:"
        echo "$RESPONSE"
        ;;
        
    3)
        echo "Forcing auto-marking via settings API..."
        RESPONSE=$(curl -s -X POST "https://connect.hsannu.com/api/settings/auto-mark?run_now=true" \
             -H "Content-Type: application/json" \
             -d "{}")
        echo "Response:"
        echo "$RESPONSE"
        ;;
        
    4)
        read -p "Enter student ID: " STUDENT_ID
        read -p "Enter date (YYYY-MM-DD) or leave blank for today: " DATE
        
        if [ -z "$DATE" ]; then
            DATE=$(date -u +"%Y-%m-%d")
        fi
        
        echo "Marking arrival for student ID $STUDENT_ID on date $DATE..."
        RESPONSE=$(curl -s -X POST "https://connect.hsannu.com/api/attendance/mark-arrival" \
             -H "Content-Type: application/json" \
             -d "{\"student_id\": $STUDENT_ID, \"date\": \"$DATE\"}")
        echo "Response:"
        echo "$RESPONSE"
        ;;
        
    *)
        echo "Invalid option. Exiting."
        exit 1
        ;;
esac

echo ""
echo "=========================================="
echo "✅ Test completed!"
echo "==========================================" 