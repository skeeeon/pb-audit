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
}

// newLogger creates a new audit logger instance.
func newLogger(app *pocketbase.PocketBase, options Options) *logger {
	return &logger{
		app:     app,
		options: options,
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

	// Store before state if available
	if beforeRecord != nil {
		beforeJSON, err := json.Marshal(beforeRecord)
		if err != nil {
			if l.options.LogToConsole {
				fmt.Printf("⚠️  WARNING Failed to marshal before state: %v\n", err)
			}
		} else {
			auditRecord.Set(AuditLogFields.BeforeChanges, beforeJSON)
		}
	}

	// Store after state if available
	if afterRecord != nil {
		afterJSON, err := json.Marshal(afterRecord)
		if err != nil {
			if l.options.LogToConsole {
				fmt.Printf("⚠️  WARNING Failed to marshal after state: %v\n", err)
			}
		} else {
			auditRecord.Set(AuditLogFields.AfterChanges, afterJSON)
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
