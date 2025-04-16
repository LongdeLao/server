#!/bin/bash

# Test script for sending a Live Activity update using curl
# This simulates what your server code is doing when it sends a notification

# Replace these values with your actual values
DEVICE_TOKEN="80b356f4805f1846b9e436530135ecb21fe2bfda5eb81041e3785d6137b3edf58981a2e3b1154799d87ff8a6f53e29b0a4d007fb630d85fdb121ab843bee5f3cbc457e9ae9cdb7c8977cca8a969c02c4"
ACTIVITY_ID="E0D62919-9EF2-49C6-A108-85A227AF3904"
BUNDLE_ID="com.leo.hsannu.push-type.liveactivity"

# Current timestamp
TIMESTAMP=$(date +%s)

# Create the JSON payload
cat > payload.json << EOF
{
  "aps": {
    "timestamp": $TIMESTAMP,
    "attributes-type": "LeaveRequestAttributes",
    "attributes": {
      "id": "$ACTIVITY_ID",
      "studentName": "John Doe",
      "studentId": 1427,
      "requestTime": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
      "reason": "Test reason"
    },
    "content-state": {
      "status": "approved",
      "responseTime": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
      "respondedBy": "Test Staff"
    },
    "event": "update",
    "alert": {
      "title": "Leave Request Update",
      "body": "Your leave request status has been updated."
    }
  },
  "activity-id": "$ACTIVITY_ID"
}
EOF

echo "📋 Generated payload:"
cat payload.json
echo ""

echo "💡 Device token length: ${#DEVICE_TOKEN} characters"
echo "💡 Activity ID: $ACTIVITY_ID"
echo "💡 Bundle ID: $BUNDLE_ID"
echo ""

echo "This is just a demonstration script. In a real implementation, you would use:"
echo "1. The APNs Provider API (HTTP/2)"
echo "2. A valid JWT token signed with your private key"
echo "3. The development APNs server during testing: api.sandbox.push.apple.com:443"
echo "4. The production APNs server for production: api.push.apple.com:443"
echo ""
echo "For detailed information on the APNs HTTP/2 API, see Apple's documentation:"
echo "https://developer.apple.com/documentation/usernotifications/setting_up_a_remote_notification_server/sending_notification_requests_to_apns"
echo ""
echo "For Live Activity specific documentation, see:"
echo "https://developer.apple.com/documentation/activitykit/updating-live-activities-with-remote-push-notifications" 