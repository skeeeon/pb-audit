# PocketBase Audit Logging System

This application extends PocketBase with comprehensive audit logging capabilities that track all record operations across all collections, including database events, API requests, and authentication events. It captures both the before and after states of records to provide a complete picture of changes.

## Features

- Logs all database operations (create, update, delete)
- Logs all API requests (create, update, delete)
- Logs authentication events
- Captures the authenticated user making the request
- Records IP address and other request details
- Stores both the before and after states of records for complete change tracking
- Prevents recursive logging by ignoring events on the audit_logs collection itself
- Automatically creates the audit_logs collection if it doesn't exist

## Prerequisites

1. PocketBase installed
2. Go 1.16 or higher

## Installation

1. Clone this repository
2. Run the application:

```bash
go run *.go
```

## How It Works

The application registers multiple hooks to capture different types of operations:

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

When an event occurs, the application:
1. Checks if the event is on the audit_logs collection (to avoid recursion)
2. Loads the original record for update operations to capture the "before" state
3. Extracts relevant information (user ID, IP address, request details, etc.)
4. Creates a new audit log record with details about the event
5. Saves both the before and after states when appropriate
6. Saves the audit log to the `audit_logs` collection

## audit_logs Collection

The application automatically creates an `audit_logs` collection with the following fields:

- `event_type` (Select): Type of event
- `collection_name` (Text): Name of the collection where the event occurred
- `record_id` (Text): ID of the affected record
- `user_id` (Text): ID of the user who performed the action
- `auth_method` (Text): Authentication method used (for auth events)
- `request_method` (Text): HTTP method (GET, POST, PUT, DELETE)
- `request_ip` (Text): IP address of the client making the request
- `request_url` (Text): URL path of the request
- `timestamp` (Date): When the event occurred
- `before_changes` (Text): JSON string snapshot of the record data before the change
- `after_changes` (Text): JSON string snapshot of the record data after the change
- `created` (Date): Auto-generated creation timestamp
- `updated` (Date): Auto-generated update timestamp

The collection is configured with admin-only access rules for security.

## Usage

Simply run the application instead of your regular PocketBase binary. All operations will be automatically logged to the `audit_logs` collection.

You can query the audit logs through the PocketBase Admin UI or API to review:
- Who made changes to which records
- When changes were made
- What data was changed (both before and after states)
- Which IP address the request came from
- Authentication events

This provides a comprehensive audit trail for security and compliance purposes.

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
