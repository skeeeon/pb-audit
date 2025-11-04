# PocketBase Audit Logging (pb-audit)

A comprehensive, production-ready audit logging library for [PocketBase](https://pocketbase.io/) applications. Track all database operations, API requests, and authentication events with complete before/after state tracking.

## Features

- 📝 **Dual-tracking system**: Captures both user intent (requests) and actual results (commits)
- 🔄 **Complete change history**: Before and after states for all operations
- 👤 **User attribution**: Tracks who performed each action
- 🌐 **Request metadata**: IP addresses, HTTP methods, URLs, and more
- 🔐 **Authentication events**: Login tracking with auth method details
- 🛡️ **Recursion prevention**: Automatically skips logging on audit collection itself
- 🚀 **Auto-setup**: Creates collection and indexes automatically
- ⚙️ **Non-destructive**: Preserves your customizations after initial setup
- 🎯 **Flexible filtering**: Optional custom logic to control what gets logged
- 📊 **Optimized queries**: Composite indexes for common query patterns

## Installation

```bash
go get github.com/skeeeon/pb-audit
```

## Quick Start

```go
package main

import (
    "log"
    "github.com/pocketbase/pocketbase"
    "github.com/skeeeon/pb-audit"
)

func main() {
    app := pocketbase.New()
    
    // Setup audit logging with default options
    if err := pbaudit.Setup(app, pbaudit.DefaultOptions()); err != nil {
        log.Fatalf("Failed to setup audit logging: %v", err)
    }
    
    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

## Understanding the Dual-Tracking System

pb-audit uses a unique **dual-tracking approach** that provides complete visibility into operations:

### Request Events (Before Commit)
- Captured when API request is received
- Include **before state** for updates/deletes
- Include request context: IP, user, HTTP method, URL
- May not complete if validation fails

**Event Types:** `create_request`, `update_request`, `delete_request`

### Success Events (After Commit)
- Captured when database operation succeeds
- Confirm operation **committed to database**
- Include **final state** after all hooks/validations
- Guarantee operation completed

**Event Types:** `create`, `update`, `delete`

### Admin/Superuser Operations

**Important:** When admins perform operations through the PocketBase Admin UI:
- Success events are always logged (confirming database operations)
- Request events are logged with `user` field as `null` (admins aren't in users collection)
- Auth events for admin login are NOT logged (only regular user authentication)

This is by design - admins/superusers are stored separately from regular users and cannot be linked via the user relation field.

### Why Both?

This dual approach answers different questions:

- **"What did the user try to do?"** → Request events
- **"What actually happened?"** → Success events
- **"Why did it fail?"** → Request event exists, no success event

Example timeline for updating a record:
1. `update_request` - User submitted changes via API (before state captured)
2. Validation runs
3. Business logic hooks execute
4. `update` - Database commit succeeded (after state captured)

## Configuration Options

Customize audit logging behavior:

```go
options := pbaudit.DefaultOptions()

// Custom collection name (default: "audit_logs")
options.CollectionName = "my_custom_audit_logs"

// Disable specific event types
options.LogAuthEvents = false      // Don't log authentication events
options.LogSuccessEvents = false   // Only log request events

// Custom event filtering
options.EventFilter = func(collectionName, eventType string) bool {
    // Only log events for sensitive collections
    if collectionName == "users" || collectionName == "payments" {
        return true
    }
    
    // Or filter by event type
    // return eventType == "delete" || eventType == "delete_request"
    
    return false
}

// Disable console logging
options.LogToConsole = false

if err := pbaudit.Setup(app, options); err != nil {
    log.Fatal(err)
}
```

## Audit Logs Collection

The library automatically creates an `audit_logs` collection with these fields:

| Field | Type | Description |
|-------|------|-------------|
| `event_type` | Select | Type of operation (create, update, delete, etc.) |
| `collection_name` | Text | Collection where event occurred |
| `record_id` | Text (optional) | ID of the affected record (empty for create_request events) |
| `user` | Relation → users (optional) | User who performed the action (null for admin/superuser actions) |
| `auth_method` | Text | Authentication method (for auth events) |
| `request_method` | Text | HTTP method (GET, POST, PUT, DELETE) |
| `request_ip` | Text | Client IP address |
| `request_url` | Text | URL path of the request |
| `timestamp` | Date | When the event occurred |
| `before_changes` | JSON | Record state before operation |
| `after_changes` | JSON | Record state after operation |
| `created` | Date | Auto-generated creation timestamp |
| `updated` | Date | Auto-generated update timestamp |

### Key Design Decisions

**User Field is Optional:**
- Relation to `users` collection
- `null` for admin/superuser actions (admins are not in users collection)
- `CascadeDelete: false` - audit logs survive user deletion
- Only set for regular user authentication

**Record ID is Optional:**
- Empty for `create_request` events (record not yet saved)
- Always present for success events (record committed with ID)

**Before/After as JSON Fields:**
- Structured data instead of text strings
- Efficient querying and parsing
- 2MB size limit per field

**Admin-Only Access (Default):**
- List, view, create, update, delete: admin only
- Prevents users from tampering with audit logs
- Can be customized after initial setup

## Change Tracking Matrix

| Event Type | Before State | After State | Record ID | User | Request Metadata |
|------------|--------------|-------------|-----------|------|------------------|
| create_request | ❌ | ✅ | ❌ (not yet saved) | ✅* | ✅ (IP, user, method, URL) |
| create | ❌ | ✅ | ✅ | ⚠️ | ❌ |
| update_request | ✅ | ✅ | ✅ | ✅* | ✅ (IP, user, method, URL) |
| update | ❌ | ✅ | ✅ | ⚠️ | ❌ |
| delete_request | ✅ | ❌ | ✅ | ✅* | ✅ (IP, user, method, URL) |
| delete | ✅ | ❌ | ✅ | ⚠️ | ❌ |
| auth | ❌ | ✅ | ✅ | ✅ | ✅ (IP, method, auth_method) |

**Legend:**
- ✅ = Always present
- ❌ = Not available
- ⚠️ = May be null (not tracked for success events)
- ✅* = Present for regular users, null for admin/superuser operations

## Usage Examples

### Query Audit Logs via API

```javascript
// JavaScript/TypeScript example

// Get recent audit logs
const logs = await pb.collection('audit_logs').getList(1, 50, {
    sort: '-timestamp'
});

// Find all changes to a specific record
const recordHistory = await pb.collection('audit_logs').getList(1, 100, {
    filter: 'record_id = "RECORD_ID"',
    sort: '-timestamp'
});

// Track user activity
const userActivity = await pb.collection('audit_logs').getList(1, 100, {
    filter: 'user = "USER_ID"',
    sort: '-timestamp',
    expand: 'user'
});

// Find all deletions
const deletions = await pb.collection('audit_logs').getList(1, 50, {
    filter: 'event_type = "delete" || event_type = "delete_request"',
    sort: '-timestamp'
});

// Filter by collection and date range
const recentUserChanges = await pb.collection('audit_logs').getList(1, 50, {
    filter: 'collection_name = "users" && timestamp >= "2024-01-01 00:00:00"',
    sort: '-timestamp'
});
```

### Advanced Filtering

```go
options := pbaudit.DefaultOptions()

// Example 1: Only log specific collections
options.EventFilter = func(collectionName, eventType string) bool {
    sensitiveCollections := []string{"users", "payments", "orders"}
    for _, col := range sensitiveCollections {
        if col == collectionName {
            return true
        }
    }
    return false
}

// Example 2: Only log destructive operations
options.EventFilter = func(collectionName, eventType string) bool {
    return eventType == "delete" || 
           eventType == "delete_request" || 
           eventType == "update" || 
           eventType == "update_request"
}

// Example 3: Skip temporary collections
options.EventFilter = func(collectionName, eventType string) bool {
    return !strings.HasPrefix(collectionName, "temp_")
}

if err := pbaudit.Setup(app, options); err != nil {
    log.Fatal(err)
}
```

## IP Address Extraction

pb-audit handles complex proxy scenarios with intelligent IP extraction:

**Priority Order:**
1. `CF-Connecting-IP` - Cloudflare (most reliable behind CDN)
2. `X-Forwarded-For` - Standard proxy (takes first/original IP)
3. `X-Real-IP` - Nginx and reverse proxies
4. `Fly-Client-IP` - Fly.io platform

**Security Note:** X-Forwarded-For can be spoofed. In production behind a trusted reverse proxy, ensure your proxy is configured correctly.

## Non-Destructive Setup

pb-audit follows a **non-destructive philosophy**:

✅ **First Setup:**
- Creates `audit_logs` collection
- Sets default API rules (admin-only)
- Creates indexes

✅ **Subsequent Starts:**
- Detects existing collection
- Skips schema modifications
- Preserves your custom API rules
- Always registers hooks

This means you can:
- Modify API rules without them being overwritten
- Add custom fields to audit logs
- Change indexes as needed
- Update collection settings

**The hooks always register**, ensuring audit logging continues even if the collection was modified.

## Performance Considerations

### Indexes

The collection includes optimized indexes for common queries:

```sql
-- Single column indexes
CREATE INDEX idx_audit_collection_name ON audit_logs (collection_name)
CREATE INDEX idx_audit_record_id ON audit_logs (record_id)
CREATE INDEX idx_audit_timestamp ON audit_logs (timestamp)
CREATE INDEX idx_audit_user ON audit_logs (user)
CREATE INDEX idx_audit_event_type ON audit_logs (event_type)

-- Composite indexes for common patterns
CREATE INDEX idx_audit_collection_timestamp ON audit_logs (collection_name, timestamp)
CREATE INDEX idx_audit_user_timestamp ON audit_logs (user, timestamp)
```

### Error Handling

Audit logging failures **never block** your application:
- Errors are logged to console (if enabled)
- Operations continue normally
- This ensures audit logging doesn't impact user experience

### Storage Considerations

- Each audit log can store up to 2MB of data per state field
- Consider implementing cleanup for old logs
- Archive or delete logs based on your retention policy

## Maintenance

### Cleaning Old Logs

Create a scheduled task to clean up old audit logs:

```javascript
// JavaScript example - run periodically (cron, etc.)
const sixMonthsAgo = new Date();
sixMonthsAgo.setMonth(sixMonthsAgo.getMonth() - 6);

const oldLogs = await pb.collection('audit_logs').getFullList({
    filter: `timestamp < "${sixMonthsAgo.toISOString()}"`
});

for (const log of oldLogs) {
    await pb.collection('audit_logs').delete(log.id);
}
```

### Tracking Admin Operations

If you need to identify which admin performed an operation, you have a few options:

**Option 1: Check request_ip field**
Admins operations will have null `user` but will have `request_ip` populated for request events.

**Option 2: Add a custom admin tracking field**
After initial setup, you can add a text field to track admin ID:
```javascript
// In PocketBase Admin UI → Collections → audit_logs:
// Add field: name="admin_email", type="text"
```

Then modify your audit hook to capture admin info (requires custom PocketBase setup).

**Option 3: Filter by null user**
```javascript
// Find all operations by admins (user is null)
const adminOps = await pb.collection('audit_logs').getList(1, 50, {
    filter: 'user = null',
    sort: '-timestamp'
});
```

### Custom API Rules

After setup, you can modify API rules for your needs:

```javascript
// Example: Allow users to view their own audit logs
// In PocketBase Admin UI → Collections → audit_logs → API Rules:

// List Rule:
// @request.auth.type = 'admin' || user.id = @request.auth.id

// View Rule:
// @request.auth.type = 'admin' || user.id = @request.auth.id
```

## Contributing

Contributions are welcome! Please follow the "grug brained developer" philosophy:
- Simple, explicit code
- Clear documentation
- One file, one purpose
- Comprehensive comments

## License

MIT License - see LICENSE file for details.

## Version

Current version: 2.0.0

**Changes from 1.x:**
- Restructured to `internal/audit/` package
- Changed `user_id` from TextField to RelationField
- Changed before/after from TextField to JSONField
- Non-destructive setup (preserves customizations)
- Improved documentation
- Better hook organization
- Cleaner IP extraction
