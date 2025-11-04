// Package audit implements comprehensive audit logging for PocketBase applications.
//
// This package tracks all database operations and API requests, creating a complete
// audit trail with before/after states, user attribution, and request metadata.
package audit

// Event type constants define the types of operations that can be audited.
//
// DUAL-TRACKING SYSTEM:
// The audit system uses two types of events for complete tracking:
//
// 1. REQUEST EVENTS (before commit):
//    - Capture user intent and request context (IP, user, method, URL)
//    - Include before state for updates/deletes
//    - May not complete if validation fails
//
// 2. SUCCESS EVENTS (after commit):
//    - Confirm operation committed to database
//    - Include final state after all hooks/validations
//    - Guarantee operation succeeded
//
// This dual approach provides complete audit trail: what was attempted (request)
// and what actually happened (success).
const (
	// API Request Events (captured before database commit)
	EventTypeCreateRequest = "create_request" // User-initiated create via API
	EventTypeUpdateRequest = "update_request" // User-initiated update via API
	EventTypeDeleteRequest = "delete_request" // User-initiated delete via API

	// Database Success Events (captured after successful commit)
	EventTypeCreate = "create" // Record successfully created
	EventTypeUpdate = "update" // Record successfully updated
	EventTypeDelete = "delete" // Record successfully deleted

	// Authentication Events
	EventTypeAuth = "auth" // User authentication (login)
)

// AllEventTypes contains all supported event types for the audit log.
// This is used when creating the event_type select field in the collection.
var AllEventTypes = []string{
	EventTypeCreateRequest,
	EventTypeUpdateRequest,
	EventTypeDeleteRequest,
	EventTypeCreate,
	EventTypeUpdate,
	EventTypeDelete,
	EventTypeAuth,
}

// AuditLogFields defines the field names used in the audit logs collection.
// Using a struct provides type safety and makes refactoring easier.
//
// FIELD DESCRIPTIONS:
//   - event_type: Type of operation (see event type constants)
//   - collection_name: Name of the collection where operation occurred
//   - record_id: ID of the affected record
//   - user: Relation to users collection (who performed the action)
//   - auth_method: Authentication method used (for auth events)
//   - request_method: HTTP method (GET, POST, PUT, DELETE, etc.)
//   - request_ip: Client IP address (with reverse proxy support)
//   - request_url: URL path of the request
//   - timestamp: When the event occurred
//   - before_changes: JSON snapshot of record before operation
//   - after_changes: JSON snapshot of record after operation
//   - created: Auto-generated creation timestamp
//   - updated: Auto-generated update timestamp
var AuditLogFields = struct {
	EventType      string
	CollectionName string
	RecordID       string
	User           string
	AuthMethod     string
	RequestMethod  string
	RequestIP      string
	RequestURL     string
	Timestamp      string
	BeforeChanges  string
	AfterChanges   string
	Created        string
	Updated        string
}{
	EventType:      "event_type",
	CollectionName: "collection_name",
	RecordID:       "record_id",
	User:           "user",
	AuthMethod:     "auth_method",
	RequestMethod:  "request_method",
	RequestIP:      "request_ip",
	RequestURL:     "request_url",
	Timestamp:      "timestamp",
	BeforeChanges:  "before_changes",
	AfterChanges:   "after_changes",
	Created:        "created",
	Updated:        "updated",
}
