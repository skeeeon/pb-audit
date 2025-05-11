# PocketBase Audit Logging

A comprehensive audit logging library for [PocketBase](https://pocketbase.io/) applications that tracks all record operations across all collections, including database events, API requests, and authentication events. It captures both the before and after states of records to provide a complete picture of changes.

## Features

- 📝 Logs all database operations (create, update, delete)
- 🔄 Logs all API requests (create, update, delete)
- 🔐 Logs authentication events
- 👤 Captures the authenticated user making the request
- 🌐 Records IP address and other request details
- 📊 Stores both the before and after states of records for complete change tracking
- 🛡️ Prevents recursive logging by ignoring events on the audit logs collection itself
- 🚀 Automatically creates the audit_logs collection if it doesn't exist
- ⚙️ Highly configurable with sensible defaults

## Installation

```bash
go get github.com/yourusername/pbaudit
```

## Quick Start

Integrating audit logging into your PocketBase application is simple:

```go
package main

import (
	"log"
	"github.com/pocketbase/pocketbase"
	"github.com/yourusername/pbaudit"
)

func main() {
	// Initialize PocketBase
	app := pocketbase.New()
	
	// Setup audit logging with default options
	if err := pbaudit.Setup(app, pbaudit.DefaultOptions()); err != nil {
		log.Fatalf("Failed to setup audit logging: %v", err)
	}
	
	// Start the PocketBase app as usual
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
```

## Configuration Options

You can customize the audit logging behavior using the `Options` struct:

```go
options := pbaudit.DefaultOptions()

// Custom collection name (default is "audit_logs")
options.CollectionName = "my_custom_audit_logs"

// Disable specific event types
options.EnableAuthEvents = false // Don't log auth events

// Custom event filtering
options.EventFilter = func(collectionName, eventType string) bool {
    // Only log events for specific collections
    return collectionName == "users" || collectionName == "sensitive_data"
    
    // Or filter by event type
    // return eventType == pbaudit.EventTypeUpdate || eventType == pbaudit.EventTypeDelete
}

// Set schema path for collection import
options.SchemaPath = "./pb_schema.json"

// Turn off auto-creation of audit collection (only if you create it manually)
options.CreateAuditCollection = false

// Disable console logging of audit events
options.LogToConsole = false

// Apply the configuration
if err := pbaudit.Setup(app, options); err != nil {
    log.Fatalf("Failed to setup audit logging: %v", err)
}
```

## How It Works

The library registers multiple hooks to capture different types of operations:

### Database Operation Hooks
- `OnRecordAfterCreateSuccess()` - Captures successful record creations
- `OnRecordAfterUpdateSuccess()` - Captures successful record updates
- `OnRecordAfterDeleteSuccess()` - Captures successful record deletions

### API Request Hooks
- `OnRecordCreateRequest()` - Captures API requests to create records
- `OnRecordUpdateRequest()` - Captures API requests to update records (with before/after states)
- `OnRecordDeleteRequest()` - Captures API requests to delete records

### Authentication Hooks
- `OnRecordAuthRequest()` - Captures authentication events

When an event occurs, the library:
1. Checks if the event is on the audit_logs collection (to avoid recursion)
2. Loads the original record for update operations to capture the "before" state
3. Extracts relevant information (user ID, IP address, request details, etc.)
4. Creates a new audit log record with details about the event
5. Saves both the before and after states when appropriate
6. Saves the audit log to the audit_logs collection

## Audit Logs Collection

The library automatically creates an `audit_logs` collection with the following fields:

| Field | Type | Description |
|-------|------|-------------|
| `event_type` | Select | Type of event (create, update, delete, auth, create_request, update_request, delete_request) |
| `collection_name` | Text | Name of the collection where the event occurred |
| `record_id` | Text | ID of the affected record |
| `user_id` | Text | ID of the user who performed the action |
| `auth_method` | Text | Authentication method used (for auth events) |
| `request_method` | Text | HTTP method (GET, POST, PUT, DELETE) |
| `request_ip` | Text | IP address of the client making the request |
| `request_url` | Text | URL path of the request |
| `timestamp` | Date | When the event occurred |
| `before_changes` | Text | JSON string snapshot of the record data before the change |
| `after_changes` | Text | JSON string snapshot of the record data after the change |
| `created` | Date | Auto-generated creation timestamp |
| `updated` | Date | Auto-generated update timestamp |

The collection is configured with admin-only access rules for security.

## Change Tracking

For most operations, the system captures:

| Event Type | Before State | After State |
|------------|--------------|------------|
| create     | Not available | Captured   |
| update     | Limited*      | Captured   |
| delete     | Captured      | Not available |
| create_request | Not available | Captured |
| update_request | Captured   | Captured   |
| delete_request | Captured   | Not available |
| auth       | Not available | Captured   |

*Note: For standard database update events, the before state might have limited availability depending on the PocketBase version and hook timing.

## Usage

You can query the audit logs through the PocketBase Admin UI or API to review:
- Who made changes to which records
- When changes were made
- What data was changed (both before and after states)
- Which IP address the request came from
- Authentication events

This provides a comprehensive audit trail for security and compliance purposes.

### Example Query

To retrieve the last 10 audit logs for a specific collection:

```javascript
// JavaScript example
const result = await pb.collection('audit_logs').getList(1, 10, {
    filter: 'collection_name = "users"',
    sort: '-timestamp'
});
```

## Advanced Usage

### Filtering Events

You can implement custom filtering logic using the `EventFilter` option:

```go
options.EventFilter = func(collectionName, eventType string) bool {
    // Skip logging for temporary collections
    if strings.HasPrefix(collectionName, "temp_") {
        return false
    }
    
    // Only log important events for regular collections
    if eventType == pbaudit.EventTypeCreate || eventType == pbaudit.EventTypeDelete {
        return true
    }
    
    // Log all events for sensitive collections
    return collectionName == "users" || 
           collectionName == "financial_records" || 
           collectionName == "permissions"
}
```

### Adding Custom Data to Audit Logs

While not directly exposed through the API, you can extend the audit_logs collection with additional fields after setup to store custom metadata.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
