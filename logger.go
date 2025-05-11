package pbaudit

import (
	"encoding/json"
	"log"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// logger provides audit logging functionality
type logger struct {
	app     *pocketbase.PocketBase
	options Options
}

// newLogger creates a new audit logger instance
func newLogger(app *pocketbase.PocketBase, options Options) *logger {
	return &logger{
		app:     app,
		options: options,
	}
}

// shouldLogEvent determines if an event should be logged based on options
func (l *logger) shouldLogEvent(collectionName string, eventType string) bool {
	// Skip logging for the audit collection itself to prevent recursion
	if collectionName == l.options.CollectionName {
		return false
	}

	// Apply custom filter if provided
	if l.options.EventFilter != nil {
		return l.options.EventFilter(collectionName, eventType)
	}

	// Default is to log all events
	return true
}

// logEvent creates a new record in the audit_logs collection
// afterRecord is the state after the operation
// beforeRecord is the state before the operation (if available)
func (l *logger) logEvent(afterRecord, beforeRecord *core.Record, collectionName string, eventType string, requestInfo map[string]interface{}) error {
	// Check if we should log this event
	if !l.shouldLogEvent(collectionName, eventType) {
		return nil
	}

	// Find the audit_logs collection
	auditCollection, err := l.app.FindCollectionByNameOrId(l.options.CollectionName)
	if err != nil {
		log.Printf("Failed to find audit_logs collection '%s': %v", l.options.CollectionName, err)
		return err
	}

	// Create a new audit log record
	auditRecord := core.NewRecord(auditCollection)
	
	// Set basic audit information
	auditRecord.Set(AuditLogFields.EventType, eventType)
	auditRecord.Set(AuditLogFields.CollectionName, collectionName)
	
	// Set record ID from either before or after record
	var recordId string
	if afterRecord != nil {
		recordId = afterRecord.Id
	} else if beforeRecord != nil {
		recordId = beforeRecord.Id
	}
	auditRecord.Set(AuditLogFields.RecordID, recordId)
	
	// Set timestamp
	auditRecord.Set(AuditLogFields.Timestamp, time.Now())
	
	// Apply request information if available
	if requestInfo != nil {
		for key, value := range requestInfo {
			auditRecord.Set(key, value)
		}
	}
	
	// If no user ID is set from request info, try to get it from the records
	if auditRecord.Get(AuditLogFields.UserID) == nil {
		if afterRecord != nil {
			if userId := afterRecord.Get("user"); userId != nil {
				auditRecord.Set(AuditLogFields.UserID, userId)
			} else if userId := afterRecord.Get("created_by"); userId != nil {
				auditRecord.Set(AuditLogFields.UserID, userId)
			}
		} else if beforeRecord != nil {
			if userId := beforeRecord.Get("user"); userId != nil {
				auditRecord.Set(AuditLogFields.UserID, userId)
			} else if userId := beforeRecord.Get("created_by"); userId != nil {
				auditRecord.Set(AuditLogFields.UserID, userId)
			}
		}
	}

	// Store before record data if available
	if beforeRecord != nil {
		beforeData := make(map[string]interface{})
		beforeDataJSON, err := json.Marshal(beforeRecord)
		if err == nil {
			// Unmarshal back to a map to get all fields
			json.Unmarshal(beforeDataJSON, &beforeData)
			// Convert to JSON string
			beforeJSON, err := json.Marshal(beforeData)
			if err == nil {
				auditRecord.Set(AuditLogFields.BeforeChanges, string(beforeJSON))
			} else {
				log.Printf("Failed to marshal before changes to JSON: %v", err)
			}
		} else {
			log.Printf("Failed to marshal before record data: %v", err)
		}
	}
	
	// Store after record data if available
	if afterRecord != nil {
		afterData := make(map[string]interface{})
		afterDataJSON, err := json.Marshal(afterRecord)
		if err == nil {
			// Unmarshal back to a map to get all fields
			json.Unmarshal(afterDataJSON, &afterData)
			// Convert to JSON string
			afterJSON, err := json.Marshal(afterData)
			if err == nil {
				auditRecord.Set(AuditLogFields.AfterChanges, string(afterJSON))
			} else {
				log.Printf("Failed to marshal after changes to JSON: %v", err)
			}
		} else {
			log.Printf("Failed to marshal after record data: %v", err)
		}
	}

	// Save the audit log
	if err := l.app.Save(auditRecord); err != nil {
		log.Printf("Failed to save audit log: %v", err)
		return err
	}

	// Log to console if enabled
	if l.options.LogToConsole {
		log.Printf("Audit log created for %s event on %s record %s", 
			eventType, collectionName, recordId)
	}

	return nil
}
