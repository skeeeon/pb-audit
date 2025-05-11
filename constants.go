package pbaudit

// Event types constants for audit log entries
const (
	// Standard database operations
	EventTypeCreate = "create"
	EventTypeUpdate = "update"
	EventTypeDelete = "delete"

	// API request operations
	EventTypeCreateReq = "create_request"
	EventTypeUpdateReq = "update_request"
	EventTypeDeleteReq = "delete_request"

	// Authentication
	EventTypeAuth = "auth"

	// Default schema path
	DefaultSchemaPath = "pb_schema.json"
)

// AuditLogFields defines the field names used in the audit logs collection
var AuditLogFields = struct {
	EventType      string
	CollectionName string
	RecordID       string
	UserID         string
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
	UserID:         "user_id",
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

// All supported event types
var AllEventTypes = []string{
	EventTypeCreate,
	EventTypeUpdate,
	EventTypeDelete,
	EventTypeCreateReq,
	EventTypeUpdateReq,
	EventTypeDeleteReq,
	EventTypeAuth,
}
