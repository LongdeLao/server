#!/bin/bash

echo "=========================================="
echo "🧪 Student Arrival Marking Test"
echo "=========================================="

# Get the current UTC time and date
UTC_TIME=$(date -u +"%H:%M:%S")
UTC_DATE=$(date -u +"%Y-%m-%d")

# Get the current Shanghai time and date
SHANGHAI_TIME=$(TZ="Asia/Shanghai" date +"%H:%M:%S")
SHANGHAI_DATE=$(TZ="Asia/Shanghai" date +"%Y-%m-%d")

echo "Current UTC time:      $UTC_TIME ($UTC_DATE)"
echo "Current Shanghai time: $SHANGHAI_TIME ($SHANGHAI_DATE)"
echo "=========================================="

# Ask for student ID
read -p "Enter student ID to mark as arrived: " STUDENT_ID

if [[ ! "$STUDENT_ID" =~ ^[0-9]+$ ]]; then
    echo "Error: Student ID must be a number"
    exit 1
fi

# Optionally, ask for a specific date
read -p "Enter date (YYYY-MM-DD) or leave blank for today: " DATE
if [ -z "$DATE" ]; then
    DATE=$UTC_DATE
fi

echo ""
echo "Making API request to mark student $STUDENT_ID as arrived on $DATE..."
echo ""

# Make the API request
RESPONSE=$(curl -s -X POST "https://connect.hsannu.com/api/attendance/mark-arrival" \
     -H "Content-Type: application/json" \
     -d "{\"student_id\": $STUDENT_ID, \"date\": \"$DATE\"}")

echo "API Response:"
echo "$RESPONSE"
echo ""

# Parse the response
if [[ "$RESPONSE" == *"\"success\":true"* ]]; then
    ARRIVED_AT=$(echo $RESPONSE | grep -o '"arrived_at":"[^"]*"' | cut -d'"' -f4)
    if [ -n "$ARRIVED_AT" ]; then
        # Parse UTC time to get hours and minutes
        HOUR=$(echo $ARRIVED_AT | cut -d':' -f1)
        MINUTE=$(echo $ARRIVED_AT | cut -d':' -f2)
        
        # Convert to Shanghai time
        SHANGHAI_HOUR=$(( (HOUR + 8) % 24 ))
        
        echo "✅ Successfully marked student as arrived!"
        echo "UTC Arrival Time: $ARRIVED_AT"
        echo "Shanghai Arrival Time: $SHANGHAI_HOUR:$MINUTE"
    else
        echo "✅ Successfully marked student as arrived!"
    fi
else
    ERROR_MSG=$(echo $RESPONSE | grep -o '"message":"[^"]*"' | cut -d'"' -f4)
    echo "❌ Failed to mark student as arrived: $ERROR_MSG"
    
    # Check if student has a late status
    echo ""
    echo "Checking student's current status..."
    
    # Get current attendance record for debugging
    ATTENDANCE_RESPONSE=$(curl -s "https://connect.hsannu.com/api/attendance/student/$STUDENT_ID")
    echo "$ATTENDANCE_RESPONSE" | grep -o '"today":"[^"]*"' | cut -d'"' -f4 | xargs -I {} echo "Current status: {}"
fi

echo ""
echo "=========================================="
echo "Test completed!"
echo "==========================================" 