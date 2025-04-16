# Leave Request API Documentation

## Data Flow Overview

```
iOS App <---> Server <---> Apple Push Notification Service
```

## Leave Request Creation

When a student creates a leave request, the following data is sent to the server:

```json
{
  "student_id": 1427,            // Integer: Student's ID
  "student_name": "John Doe",    // String: Student's name
  "request_type": "early",       // String: "early", "medical", or "absence"
  "reason": "Doctor appointment", // String: Reason for the request (optional)
  "live_activity_id": "E0D62919-9EF2-49C6-A108-85A227AF3904", // String: Live Activity ID (optional)
  "live_activity_token": "80b356f4805f1846b9e436530135ecb21fe2bfda5eb81041e3785d6137b3edf58981a2e3b1154799d87ff8a6f53e29b0a4d007fb630d85fdb121ab843bee5f3cbc457e9ae9cdb7c8977cca8a969c02c4" // String: APNs token for Live Activity (optional)
}
```

### Endpoint
- `POST /api/leave-requests`

### Response
```json
{
  "success": true,
  "request": {
    "id": 22,
    "student_id": 1427,
    "student_name": "John Doe",
    "request_type": "early",
    "reason": "Doctor appointment",
    "status": "pending",
    "created_at": "2025-04-16T10:00:00Z",
    "updated_at": "2025-04-16T10:00:00Z",
    "responded_by": null,
    "response_time": null,
    "live_activity_id": "E0D62919-9EF2-49C6-A108-85A227AF3904",
    "live_activity_token": "80b356f4805f1846b9e436530135ecb21fe2bfda5eb81041e3785d6137b3edf58981a2e3b1154799d87ff8a6f53e29b0a4d007fb630d85fdb121ab843bee5f3cbc457e9ae9cdb7c8977cca8a969c02c4"
  }
}
```

## Live Activity Token Update

If the Live Activity tokens aren't available during initial request creation, they can be updated later:

```json
{
  "live_activity_id": "E0D62919-9EF2-49C6-A108-85A227AF3904",
  "live_activity_token": "80b356f4805f1846b9e436530135ecb21fe2bfda5eb81041e3785d6137b3edf58981a2e3b1154799d87ff8a6f53e29b0a4d007fb630d85fdb121ab843bee5f3cbc457e9ae9cdb7c8977cca8a969c02c4"
}
```

### Endpoint
- `PUT /api/leave-requests/:requestId/live-activity`

## Status Update by Staff

When staff updates a leave request status:

```json
{
  "status": "approved",      // String: "approved", "rejected", or "finished"
  "staff_id": 101,           // Integer: Staff member's ID
  "staff_name": "Jane Smith" // String: Staff member's name
}
```

### Endpoint
- `PUT /api/leave-requests/:requestId/status`

## APNs Notification for Live Activity

When a leave request status is updated, the server sends the following payload to Apple's APNs service:

```json
{
  "aps": {
    "timestamp": 1714042800,
    "attributes-type": "LeaveRequestAttributes",
    "attributes": {
      "id": "E0D62919-9EF2-49C6-A108-85A227AF3904",
      "studentName": "John Doe",
      "studentId": 1427,
      "requestTime": "2025-04-16T12:00:00Z",
      "reason": "Doctor's appointment"
    },
    "content-state": {
      "status": "approved",
      "responseTime": "2025-04-16T10:04:52Z",
      "respondedBy": "Jane Smith"
    },
    "event": "update",
    "alert": {
      "title": "Leave Request Update",
      "body": "Your leave request status has been updated."
    }
  },
  "activity-id": "E0D62919-9EF2-49C6-A108-85A227AF3904"
}
```

### Important Notes

1. The `activity-id` field **must** be at the top level of the JSON (not nested inside `aps`)
2. The `content-state` must match the structure expected by the iOS app's `ContentState` struct
3. The APNs device token for Live Activities is approximately 160 characters long (much longer than regular APNs tokens)
4. The payload must include `attributes-type` and the full `attributes` structure that mirrors the initial Live Activity
5. Include an `alert` object with `title` and `body` for better user experience
6. The APNs topic must be your app's bundle identifier with `.push-type.liveactivity` appended
7. Set Development environment for testing

## Live Activity Data Model

The iOS app's Live Activity model that receives this data:

```swift
struct LeaveRequestAttributes: ActivityAttributes {
    public struct ContentState: Codable, Hashable {
        var status: String           // "sent", "pending", "approved", "rejected", "finished"
        var responseTime: Date?      // When the staff responded
        var respondedBy: String?     // Name of the staff member
    }

    let id: String                  // The leave request ID
    let studentName: String         // Student's name
    let studentId: Int              // Student's ID
    let requestTime: Date           // When the request was created
    let reason: String              // Reason for the request
}
```

## Debugging Tips

1. Verify the token length is approximately 160 characters
2. Do NOT trim or modify the token - use it exactly as received from the device
3. Ensure the activity-id matches between client and server
4. Include the full attributes structure in the payload
5. Check that content-state fields match exactly between server and iOS app
6. The bundle ID must be correct in the APNs request
7. Use Development APNs server during testing
8. Verify the device has an active internet connection 