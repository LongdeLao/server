# HSANNU Server Status Configuration

## Overview
The HSANNU server now supports interactive status configuration on startup. When you start the server, it will prompt you to select the current server status.

## Starting the Server

1. Navigate to the server directory:
   ```bash
   cd server
   ```

2. Run the server:
   ```bash
   go run main.go
   ```
   or build and run:
   ```bash
   go build -o server .
   ./server
   ```

3. You'll see an interactive prompt:
   ```
   🚀 HSANNU Server Configuration
   ==============================

   Please select the server status:
     (a) Active - Server is running normally
     (m) Maintenance - Server is under maintenance
     (c) Construction - Server is under construction

   Enter your choice [a/m/c] (default: a):
   ```

## Status Options

### Active (a)
- **Status**: `active`
- **Default Message**: "Server is running normally"
- **HTTP Response**: 200 OK
- **Use Case**: Normal operation

### Maintenance (m)
- **Status**: `maintenance`
- **Default Message**: "Server is under maintenance. Please try again later."
- **HTTP Response**: 503 Service Unavailable
- **Use Case**: Scheduled maintenance, updates, or temporary downtime
- **Custom Message**: You can enter a custom maintenance message when prompted

### Construction (c)
- **Status**: `construction`
- **Default Message**: "Server is under construction. New features are being added."
- **HTTP Response**: 503 Service Unavailable
- **Use Case**: Major updates, new feature development, or significant changes
- **Custom Message**: You can enter a custom construction message when prompted

## API Endpoint

The server status can be checked via:
```
GET /api/check-status
```

### Response Format
```json
{
  "status": "active|maintenance|construction",
  "message": "Status message",
  "timestamp": "2024-01-01T12:00:00Z",
  "version": "1.0.0",
  "uptime": "2h 30m 45s",
  "is_active": true|false,
  "environment": "production|maintenance|development"
}
```

### HTTP Status Codes
- **200 OK**: Server is active and operational
- **503 Service Unavailable**: Server is under maintenance or construction
- **502 Bad Gateway**: Server is offline or unreachable

## iOS App Integration

The iOS app will automatically check the server status after the splash screen:

- **Active**: Proceeds normally to login/main view
- **Maintenance/Construction**: Shows status screen with retry option
- **Offline/Error**: Shows connection error with retry option

The status check happens automatically and users only see the status screen when there are issues.

## Examples

### Starting with Active Status
```
Enter your choice [a/m/c] (default: a): a
✅ Server status set to: ACTIVE
📝 Status Message: Server is running normally
```

### Starting with Maintenance
```
Enter your choice [a/m/c] (default: a): m
🔧 Server status set to: MAINTENANCE
Enter custom maintenance message (optional): Database migration in progress
📝 Status Message: Database migration in progress
```

### Starting with Construction
```
Enter your choice [a/m/c] (default: a): c
🚧 Server status set to: CONSTRUCTION
Enter custom construction message (optional): Adding new AI features
📝 Status Message: Adding new AI features
``` 