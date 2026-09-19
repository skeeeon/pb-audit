package audit

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// logger provides audit logging functionality.
type logger struct {
	app     *pocketbase.PocketBase
	options Options

	// snapshot is Options.SnapshotCollections as a set. Built once here rather
	// than scanned per event: this is consulted on every audited write.
	snapshot map[string]bool
}

// newLogger creates a new audit logger instance.
func newLogger(app *pocketbase.PocketBase, options Options) *logger {
	snapshot := make(map[string]bool, len(options.SnapshotCollections))
	for _, name := range options.SnapshotCollections {
		snapshot[name] = true
	}

	return &logger{
		app:      app,
		options:  options,
		snapshot: snapshot,
	}
}

// shouldLogEvent determines if an event should be logged based on options.
//
// FILTERING RULES:
// 1. Never log events on the audit collection itself (prevents recursion)
// 2. Apply custom EventFilter if provided
// 3. Default: log all events
//
// PARAMETERS:
//   - collectionName: Name of the collection where event occurred
//   - eventType: Type of event (see event type constants)
//
// RETURNS:
//   - true if event should be logged
//   - false if event should be skipped
func (l *logger) shouldLogEvent(collectionName string, eventType string) bool {
	// Never log events on the audit collection itself to prevent recursion
	if collectionName == l.options.CollectionName {
		return false
	}

	// Apply custom filter if provided
	if l.options.EventFilter != nil {
		return l.options.EventFilter(collectionName, eventType)
	}

	// Default: log all events
	return true
}

// logEvent creates a new audit log record.
//
// This is the core logging function that handles all event types.
// It captures the before/after states and request metadata.
//
// PARAMETERS:
//   - afterRecord: Record state after operation (nil for delete)
//   - beforeRecord: Record state before operation (nil for create)
//   - collectionName: Name of collection where operation occurred
//   - eventType: Type of event (see event type constants)
//   - requestInfo: Map of request metadata (user, IP, method, URL, etc.)
//
// RETURNS:
//   - nil on success
//   - error if audit log creation fails (logged but doesn't block operation)
func (l *logger) logEvent(
	afterRecord *core.Record,
	beforeRecord *core.Record,
	collectionName string,
	eventType string,
	requestInfo map[string]interface{},
) error {
	// Check if we should log this event
	if !l.shouldLogEvent(collectionName, eventType) {
		return nil
	}

	// Find the audit logs collection
	auditCollection, err := l.app.FindCollectionByNameOrId(l.options.CollectionName)
	if err != nil {
		if l.options.LogToConsole {
			fmt.Printf("⚠️  WARNING Failed to find audit logs collection: %v\n", err)
		}
		return err
	}

	// Create new audit log record
	auditRecord := core.NewRecord(auditCollection)

	// Set basic audit information
	auditRecord.Set(AuditLogFields.EventType, eventType)
	auditRecord.Set(AuditLogFields.CollectionName, collectionName)
	auditRecord.Set(AuditLogFields.Timestamp, time.Now())

	// Set record ID from either before or after record
	var recordID string
	if afterRecord != nil {
		recordID = afterRecord.Id
	} else if beforeRecord != nil {
		recordID = beforeRecord.Id
	}
	// Only set if not empty (create_request events may not have ID yet)
	if recordID != "" {
		auditRecord.Set(AuditLogFields.RecordID, recordID)
	}

	// Apply request information if available
	if requestInfo != nil {
		for key, value := range requestInfo {
			// Special handling for user field - only set if it's a valid user ID
			if key == AuditLogFields.User {
				userID, ok := value.(string)
				if ok && userID != "" {
					// Verify the user exists in the users collection before setting relation
					if l.isValidUser(userID) {
						auditRecord.Set(key, value)
					}
					// If not valid, skip setting it (admin/superuser case)
				}
			} else {
				auditRecord.Set(key, value)
			}
		}
	}

	// The two states as PocketBase itself would serialise them. Record.MarshalJSON
	// is json.Marshal(record.PublicExport()), so exporting once here and marshalling
	// the map below stores byte-identical JSON to what this used to write, while
	// also giving the field diff something to compare.
	before := publicExport(beforeRecord)
	after := publicExport(afterRecord)

	// Which fields moved. Recorded for every record event, whether or not the
	// collection stores values -- this is the part that is always safe to keep.
	// Skipped for auth events, where there is no before state and "every
	// populated field of the user record" is noise rather than a change.
	if eventType != EventTypeAuth {
		auditRecord.Set(AuditLogFields.ChangedFields, changedFields(before, after))
	}

	// Values, only for collections that opted in. See changedFields for why
	// this is an allowlist and why the default is names only.
	if l.snapshot[collectionName] {
		if before != nil {
			l.setSnapshot(auditRecord, AuditLogFields.BeforeChanges, before, "before")
		}
		if after != nil {
			l.setSnapshot(auditRecord, AuditLogFields.AfterChanges, after, "after")
		}
	}

	// Save the audit log
	if err := l.app.Save(auditRecord); err != nil {
		if l.options.LogToConsole {
			fmt.Printf("⚠️  WARNING Failed to save audit log: %v\n", err)
		}
		return err
	}

	// Log to console if enabled
	if l.options.LogToConsole {
		fmt.Printf("📝 AUDIT %s event on %s record %s\n", eventType, collectionName, recordID)
	}

	return nil
}

// publicExport returns the record as the map PocketBase would serialise, or nil
// when there is no record for that side of the event.
//
// Hidden fields are already excluded by PublicExport, along with password and
// tokenKey on auth collections. Everything else the application chose not to
// hide is in here -- which is precisely why the values are not stored by
// default. See changedFields.
func publicExport(record *core.Record) map[string]any {
	if record == nil {
		return nil
	}
	return record.PublicExport()
}

// setSnapshot marshals one state into the audit record, logging rather than
// failing if it will not serialise. Audit logging never blocks the operation it
// is describing.
func (l *logger) setSnapshot(auditRecord *core.Record, field string, state map[string]any, label string) {
	encoded, err := json.Marshal(state)
	if err != nil {
		if l.options.LogToConsole {
			fmt.Printf("⚠️  WARNING Failed to marshal %s state: %v\n", label, err)
		}
		return
	}
	auditRecord.Set(field, encoded)
}

// isValidUser checks if a user ID exists in the users collection.
//
// This is necessary because authenticated users might be admins/superusers
// who are not in the regular users collection. We only want to set the
// user relation field if it's a valid user record.
//
// PARAMETERS:
//   - userID: User ID to check
//
// RETURNS:
//   - true if user exists in users collection
//   - false if user doesn't exist (e.g., admin/superuser)
func (l *logger) isValidUser(userID string) bool {
	_, err := l.app.FindRecordById("users", userID)
	return err == nil
}

// extractClientIP attempts to determine the real client IP address.
//
// This function checks headers in order of reliability for common hosting scenarios.
// It handles cases where the application is behind reverse proxies, CDNs, or load balancers.
//
// PRIORITY ORDER:
// 1. CF-Connecting-IP: Cloudflare's real IP (most reliable behind CDN)
// 2. X-Forwarded-For: Standard proxy header (takes first/original IP)
// 3. X-Real-IP: Nginx and other reverse proxies
// 4. Fly-Client-IP: Fly.io platform header
//
// SECURITY NOTE:
// X-Forwarded-For can be spoofed. In production behind a trusted reverse proxy,
// consider validating the proxy chain or using more specific headers.
//
// PARAMETERS:
//   - reqInfo: Request information containing headers
//
// RETURNS:
//   - Client IP address
//   - "unknown" if IP cannot be determined
func extractClientIP(reqInfo *core.RequestInfo) string {
	if reqInfo == nil {
		return "unknown"
	}

	// Normalize headers to lowercase for case-insensitive lookup
	headers := make(map[string]string)
	for k, v := range reqInfo.Headers {
		headers[strings.ToLower(k)] = v
	}

	// Check Cloudflare
	if ip := headers["cf-connecting-ip"]; ip != "" {
		return ip
	}

	// Check X-Forwarded-For (take first IP = original client)
	if xff := headers["x-forwarded-for"]; xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP
	if ip := headers["x-real-ip"]; ip != "" {
		return ip
	}

	// Check Fly.io
	if ip := headers["fly-client-ip"]; ip != "" {
		return ip
	}

	return "unknown"
}
