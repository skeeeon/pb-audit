# PocketBase Audit Logging (pb-audit)

A comprehensive, production-ready audit logging library for [PocketBase](https://pocketbase.io/) applications. Track all database operations, API requests, and authentication events with complete before/after state tracking.

## Features

- 📝 **Dual-tracking system**: Captures both user intent (requests) and actual results (commits)
- 🔄 **Complete change history**: Every event names the fields that changed; full before/after values are opt-in per collection
- 🔒 **Safe by default**: Values are not copied into the audit log unless you ask for them, so a field left readable for its owner does not become a permanent second copy
- 👤 **User attribution**: Tracks who performed each action
- 🌐 **Request metadata**: IP addresses, HTTP methods, URLs, and more
- 🔐 **Authentication events**: Login tracking with auth method details
- 🛡️ **Recursion prevention**: Automatically skips logging on audit collection itself
- 🚀 **Auto-setup**: Creates collection and indexes automatically
- ⚙️ **Non-destructive**: Preserves your customizations after initial setup
- 🎯 **Flexible filtering**: Optional custom logic to control what gets logged
- 🧹 **Retention policies**: Automatic cleanup by age or record count on a cron schedule
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

// Record full before/after VALUES for these collections. Everything else
// records only changed_fields -- the NAMES of the fields that moved.
// The default (nil) stores no values at all; see "What gets recorded" below.
options.SnapshotCollections = []string{"products", "memberships"}

// Disable console logging
options.LogToConsole = false

// Automatic retention policy
options.Retention = &pbaudit.RetentionPolicy{
    MaxAge:     90 * 24 * time.Hour, // Delete logs older than 90 days
    MaxRecords: 100000,              // Keep at most 100k records
    Interval:   "0 2 * * *",        // Run cleanup at 2 AM daily
}

if err := pbaudit.Setup(app, options); err != nil {
    log.Fatal(err)
}
```

## Retention Policy

pb-audit can automatically clean up old audit logs on a schedule using PocketBase's built-in cron scheduler. Configure a retention policy to keep your audit collection bounded without external scripts.

```go
options := pbaudit.DefaultOptions()
options.Retention = &pbaudit.RetentionPolicy{
    MaxAge:     30 * 24 * time.Hour, // Delete records older than 30 days
    MaxRecords: 50000,               // Keep at most 50k records
    Interval:   "0 0 * * *",        // Run daily at midnight (default)
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `MaxAge` | `time.Duration` | `0` (disabled) | Delete records older than this duration |
| `MaxRecords` | `int` | `0` (disabled) | Keep at most this many records (oldest deleted first) |
| `Interval` | `string` | `"0 0 * * *"` | Cron expression for cleanup schedule |

**Behavior:**
- Both constraints can be used independently or together — when both are set, both are enforced
- If neither `MaxAge` nor `MaxRecords` is set, no cleanup job is registered
- Deletion happens in batches to avoid excessive memory usage
- Cleanup errors are logged but never block the application

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
| `changed_fields` | JSON | Names of the fields that differ between the two states. Always recorded |
| `before_changes` | JSON | Record state before operation. Only for collections in `SnapshotCollections` |
| `after_changes` | JSON | Record state after operation. Only for collections in `SnapshotCollections` |
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
- Populated only for collections named in `SnapshotCollections`

**Field Names Always, Values On Request:**
- See "What gets recorded" below

**Admin-Only Access (Default):**
- List, view, create, update, delete: admin only
- Prevents users from tampering with audit logs
- Can be customized after initial setup

## What Gets Recorded

Every audited event records `changed_fields`: a sorted array naming the fields
whose value differs between the two states.

```json
{ "event_type": "update", "collection_name": "api_keys",
  "record_id": "rt4k9...", "changed_fields": ["rotated_at", "secret"] }
```

That answers who changed what, and when, without putting the value in the log.

`before_changes` and `after_changes` — the full values — are written only for
the collections you name in `SnapshotCollections`. **The default is nil, so no
values are stored anywhere.**

### Why values are opt-in

A snapshot copies every field of a record except the ones the collection marks
`hidden`. "Not hidden" is not the same question as "safe to keep a permanent
second copy of": applications routinely leave a credential readable so that the
identity owning it can fetch it back, and rely on row-level API rules to decide
who sees which row. The audit collection has no row scoping — it is one flat
table holding a copy of every record — so a value protected by scoping is not
protected once it is in here. It also outlives rotation: the credential you
replaced because it leaked is still sitting in `before_changes`.

Naming the fields keeps the forensic value and leaves the copy out.

### Choosing collections

List the ones whose diffs a human actually reads, and leave out anything
holding a secret:

```go
options.SnapshotCollections = []string{"products", "orders", "memberships"}
```

It is an allowlist rather than a list of fields to redact, deliberately: a deny
list stops covering a sensitive field the moment someone adds one, and does so
silently.

### How the diff is computed

- Values are compared as JSON, which is the form they would be stored in.
- **Absent and zero are the same thing.** A create therefore names the fields
  that were actually populated rather than every field the collection has.
  Genuine transitions are unaffected: `"x" → ""` and `true → false` each have a
  non-empty side.
- `collectionId`, `collectionName` and `expand` are skipped — they are export
  envelope, not fields of the record.
- Auth events get no `changed_fields`; there is no before state to compare.
- A value that will not serialise is reported as changed rather than assumed
  equal, so the trail never quietly omits a field.

### One case over-reports

`Original()` is empty for a record built in memory, so code that creates a
record and saves it *again* without reloading gives the success hook no before
state, and the diff names every populated field instead of the ones that moved.
That over-reports; it never hides a change.

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

- Each audit log can store up to 2MB of data per state field, but only for collections in `SnapshotCollections` — by default rows carry field names only, which is far smaller
- Consider implementing cleanup for old logs
- Archive or delete logs based on your retention policy

## Maintenance

### Cleaning Old Logs

The recommended approach is to use the built-in [Retention Policy](#retention-policy):

```go
options.Retention = &pbaudit.RetentionPolicy{
    MaxAge: 180 * 24 * time.Hour, // 6 months
}
```

Alternatively, you can clean up manually via the API:

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

See the git tags for released versions. There is deliberately no version
constant in the source: a library has no ldflags equivalent, so a hardcoded
string is one that drifts from the tag and can only mislead. Consumers should
read the version from `debug.ReadBuildInfo()`, which cannot.

**Unreleased:**
- Added `changed_fields`: every event names the fields that moved
- Full before/after values are now opt-in per collection via `SnapshotCollections`. **This is a behaviour change** — previously every event stored a full snapshot. Set `SnapshotCollections` to restore the old behaviour for the collections that need it
- `update` success events now carry a real before state (from `Record.Original()`), so a programmatic `app.Save()` produces a diff rather than only confirming the commit

**Changes in 2.x:**
- Restructured to `internal/audit/` package
- Changed `user_id` from TextField to RelationField
- Changed before/after from TextField to JSONField
- Non-destructive setup (preserves customizations)
- Improved documentation
- Better hook organization
- Cleaner IP extraction
