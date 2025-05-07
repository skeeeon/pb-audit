// main.go - PocketBase application with audit logging capabilities
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Constants for audit log fields and values
const (
	// Collection name
	AuditCollectionName = "audit_logs"
	
	// Event types
	EventTypeCreate       = "create"
	EventTypeUpdate       = "update"
	EventTypeDelete       = "delete"
	EventTypeAuth         = "auth"
	EventTypeCreateReq    = "create_request"
	EventTypeUpdateReq    = "update_request"
	EventTypeDeleteReq    = "delete_request"

	// Schema path
	DefaultSchemaPath = "pb_schema.json"
)

func main() {
	// Initialize PocketBase
	app := pocketbase.New()
	
	// Setup the collections and audit logging system using the OnBootstrap hook
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		// IMPORTANT: Wait for bootstrap to complete before accessing the database
		if err := e.Next(); err != nil {
			return err
		}

		// Determine schema file path - use environment variable if set, otherwise use default
		schemaPath := os.Getenv("PB_SCHEMA_PATH")
		if schemaPath == "" {
			// Use the default path relative to the executable
			execPath, err := os.Executable()
			if err != nil {
				schemaPath = DefaultSchemaPath // Fallback to current directory
			} else {
				schemaPath = filepath.Join(filepath.Dir(execPath), DefaultSchemaPath)
				
				// If the file doesn't exist at the executable path, try the current directory
				if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
					schemaPath = DefaultSchemaPath
				}
			}
		}

		// Check if schema file exists
		if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
			log.Printf("Warning: Schema file not found at %s, skipping collection setup", schemaPath)
			
			// Still setup just the audit logs collection using existing functionality
			if err := setupAuditCollection(app); err != nil {
				log.Printf("Warning: Failed to setup audit logs collection: %v", err)
			} else {
				log.Println("Audit logging system initialized successfully")
			}
		} else {
			// Initialize collections from schema using PocketBase's built-in import functionality
			if err := importCollectionsFromFile(app, schemaPath); err != nil {
				log.Printf("Warning: Failed to import collections from schema: %v", err)
				
				// Fallback to just setting up the audit collection
				if err := setupAuditCollection(app); err != nil {
					log.Printf("Warning: Failed to setup audit logs collection: %v", err)
				}
			} else {
				log.Println("Collections imported successfully from schema")
			}
		}

		// Register standard event hooks
		setupStandardEventHooks(app)
		
		// Register API request event hooks
		setupRequestEventHooks(app)
		
		// Register auth event hooks
		setupAuthEventHooks(app)
		
		return nil
	})

	// Start the application
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// importCollectionsFromFile reads a schema file and imports all collections
func importCollectionsFromFile(app *pocketbase.PocketBase, schemaPath string) error {
	// Read the schema file
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	// Import collections using PocketBase's built-in functionality
	// Setting deleteMissing to false to prevent accidental data loss
	if err := app.ImportCollectionsByMarshaledJSON(schemaData, false); err != nil {
		return err
	}

	// Verify that the audit_logs collection exists even after successful import
	_, err = app.FindCollectionByNameOrId(AuditCollectionName)
	if err != nil {
		log.Println("Audit logs collection not found in schema, creating it now...")
		if err := setupAuditCollection(app); err != nil {
			return fmt.Errorf("failed to create audit logs collection: %w", err)
		}
	}

	return nil
}

// setupStandardEventHooks registers hooks for standard database operations
func setupStandardEventHooks(app *pocketbase.PocketBase) {
	// Register hooks for record creation events
	app.OnRecordAfterCreateSuccess().BindFunc(func(e *core.RecordEvent) error {
		// Get the collection name from the record
		collectionName := e.Record.Collection().Name
		
		// Skip audit logs collection to prevent recursion
		if collectionName == AuditCollectionName {
			return e.Next()
		}
		
		// For create events, there's no "before" state
		return logAuditEvent(e.App, e.Record, nil, collectionName, EventTypeCreate, nil)
	})

	// Register hooks for record update events
	app.OnRecordAfterUpdateSuccess().BindFunc(func(e *core.RecordEvent) error {
		// Get the collection name from the record
		collectionName := e.Record.Collection().Name
		
		// Skip audit logs collection to prevent recursion
		if collectionName == AuditCollectionName {
			return e.Next()
		}
		
		// For updates through standard events, we don't have easy access to the previous state
		return logAuditEvent(e.App, e.Record, nil, collectionName, EventTypeUpdate, nil)
	})

	// Register hooks for record deletion events
	app.OnRecordAfterDeleteSuccess().BindFunc(func(e *core.RecordEvent) error {
		// Get the collection name from the record
		collectionName := e.Record.Collection().Name
		
		// Skip audit logs collection to prevent recursion
		if collectionName == AuditCollectionName {
			return e.Next()
		}
		
		// For delete events, the "after" state doesn't exist, but we have the "before" state
		return logAuditEvent(e.App, nil, e.Record, collectionName, EventTypeDelete, nil)
	})
	
	log.Println("Standard event hooks registered")
}

// setupRequestEventHooks registers hooks for API request events
func setupRequestEventHooks(app *pocketbase.PocketBase) {
	// Register hooks for record create request events
	app.OnRecordCreateRequest().BindFunc(func(e *core.RecordRequestEvent) error {
		// Skip audit logs collection to prevent recursion
		if e.Collection.Name == AuditCollectionName {
			return e.Next()
		}
		
		// Extract request information
		requestInfo := make(map[string]interface{})
		
		// Use RequestInfo method to get request details
		reqInfo, err := e.RequestInfo()
		if err != nil {
			log.Printf("Failed to get request info: %v", err)
		} else {
			requestInfo["request_method"] = reqInfo.Method
			
			// Extract IP from headers
			clientIP := reqInfo.Headers["x-forwarded-for"]
			if clientIP == "" {
				clientIP = reqInfo.Headers["x-real-ip"]
			}
			if clientIP == "" {
				clientIP = reqInfo.Headers["cf-connecting-ip"] // Cloudflare
			}
			requestInfo["request_ip"] = clientIP
			
			// Add request context as URL
			requestInfo["request_url"] = reqInfo.Context
			
			// Add authenticated user if available
			if reqInfo.Auth != nil {
				requestInfo["user_id"] = reqInfo.Auth.Id
			}
		}
		
		// For create requests, there's no "before" state
		err = logAuditEvent(e.App, e.Record, nil, e.Collection.Name, EventTypeCreateReq, requestInfo)
		if err != nil {
			log.Printf("Failed to log create request event: %v", err)
		}
		
		return e.Next()
	})
	
	// Register hooks for record update request events
	app.OnRecordUpdateRequest().BindFunc(func(e *core.RecordRequestEvent) error {
		// Skip audit logs collection to prevent recursion
		if e.Collection.Name == AuditCollectionName {
			return e.Next()
		}
		
		// Load the original record from the database to get the "before" state
		originalRecord, err := app.FindRecordById(e.Collection.Name, e.Record.Id)
		if err != nil {
			log.Printf("Failed to load original record for update tracking: %v", err)
		}
		
		// Extract request information
		requestInfo := make(map[string]interface{})
		
		// Use RequestInfo method to get request details
		reqInfo, err := e.RequestInfo()
		if err != nil {
			log.Printf("Failed to get request info: %v", err)
		} else {
			requestInfo["request_method"] = reqInfo.Method
			
			// Extract IP from headers
			clientIP := reqInfo.Headers["x-forwarded-for"]
			if clientIP == "" {
				clientIP = reqInfo.Headers["x-real-ip"]
			}
			if clientIP == "" {
				clientIP = reqInfo.Headers["cf-connecting-ip"] // Cloudflare
			}
			requestInfo["request_ip"] = clientIP
			
			// Add request context as URL
			requestInfo["request_url"] = reqInfo.Context
			
			// Add authenticated user if available
			if reqInfo.Auth != nil {
				requestInfo["user_id"] = reqInfo.Auth.Id
			}
		}
		
		// Pass both original and updated record
		err = logAuditEvent(e.App, e.Record, originalRecord, e.Collection.Name, EventTypeUpdateReq, requestInfo)
		if err != nil {
			log.Printf("Failed to log update request event: %v", err)
		}
		
		return e.Next()
	})
	
	// Register hooks for record delete request events
	app.OnRecordDeleteRequest().BindFunc(func(e *core.RecordRequestEvent) error {
		// Skip audit logs collection to prevent recursion
		if e.Collection.Name == AuditCollectionName {
			return e.Next()
		}
		
		// Extract request information
		requestInfo := make(map[string]interface{})
		
		// Use RequestInfo method to get request details
		reqInfo, err := e.RequestInfo()
		if err != nil {
			log.Printf("Failed to get request info: %v", err)
		} else {
			requestInfo["request_method"] = reqInfo.Method
			
			// Extract IP from headers
			clientIP := reqInfo.Headers["x-forwarded-for"]
			if clientIP == "" {
				clientIP = reqInfo.Headers["x-real-ip"]
			}
			if clientIP == "" {
				clientIP = reqInfo.Headers["cf-connecting-ip"] // Cloudflare
			}
			requestInfo["request_ip"] = clientIP
			
			// Add request context as URL
			requestInfo["request_url"] = reqInfo.Context
			
			// Add authenticated user if available
			if reqInfo.Auth != nil {
				requestInfo["user_id"] = reqInfo.Auth.Id
			}
		}
		
		// For delete operations, the "after" state doesn't exist, but we have the "before" state
		err = logAuditEvent(e.App, nil, e.Record, e.Collection.Name, EventTypeDeleteReq, requestInfo)
		if err != nil {
			log.Printf("Failed to log delete request event: %v", err)
		}
		
		return e.Next()
	})
	
	log.Println("Request event hooks registered")
}

// setupAuthEventHooks registers hooks for authentication events
func setupAuthEventHooks(app *pocketbase.PocketBase) {
	app.OnRecordAuthRequest().BindFunc(func(e *core.RecordAuthRequestEvent) error {
		// Extract request information
		requestInfo := make(map[string]interface{})
		
		// Add auth method
		requestInfo["auth_method"] = e.AuthMethod
		
		// Add user ID if record exists
		if e.Record != nil {
			requestInfo["user_id"] = e.Record.Id
		}
		
		// Extract request data
		reqInfo, err := e.RequestInfo()
		if err != nil {
			log.Printf("Failed to get request info: %v", err)
		} else {
			requestInfo["request_method"] = reqInfo.Method
			
			// Extract IP from headers
			clientIP := reqInfo.Headers["x-forwarded-for"]
			if clientIP == "" {
				clientIP = reqInfo.Headers["x-real-ip"]
			}
			if clientIP == "" {
				clientIP = reqInfo.Headers["cf-connecting-ip"] // Cloudflare
			}
			requestInfo["request_ip"] = clientIP
			
			// Add request context as URL
			requestInfo["request_url"] = reqInfo.Context
		}
		
		// For auth events, there's no "before" state but we still have the current state
		err = logAuditEvent(e.App, e.Record, nil, e.Record.Collection().Name, EventTypeAuth, requestInfo)
		if err != nil {
			log.Printf("Failed to log auth event: %v", err)
		}
		
		return e.Next()
	})
	
	log.Println("Auth event hooks registered")
}

// logAuditEvent creates a new record in the audit_logs collection
// afterRecord is the state after the operation
// beforeRecord is the state before the operation (if available)
func logAuditEvent(app core.App, afterRecord, beforeRecord *core.Record, collectionName string, eventType string, requestInfo map[string]interface{}) error {
	// Find the audit_logs collection
	auditCollection, err := app.FindCollectionByNameOrId(AuditCollectionName)
	if err != nil {
		log.Printf("Failed to find audit_logs collection: %v", err)
		return err
	}

	// Create a new audit log record
	auditRecord := core.NewRecord(auditCollection)
	
	// Set basic audit information
	auditRecord.Set("event_type", eventType)
	auditRecord.Set("collection_name", collectionName)
	
	// Set record ID from either before or after record
	var recordId string
	if afterRecord != nil {
		recordId = afterRecord.Id
	} else if beforeRecord != nil {
		recordId = beforeRecord.Id
	}
	auditRecord.Set("record_id", recordId)
	
	// Set timestamp
	auditRecord.Set("timestamp", time.Now())
	
	// Apply request information if available
	if requestInfo != nil {
		for key, value := range requestInfo {
			auditRecord.Set(key, value)
		}
	}
	
	// If no user ID is set from request info, try to get it from the records
	if auditRecord.Get("user_id") == nil {
		if afterRecord != nil {
			if userId := afterRecord.Get("user"); userId != nil {
				auditRecord.Set("user_id", userId)
			} else if userId := afterRecord.Get("created_by"); userId != nil {
				auditRecord.Set("user_id", userId)
			}
		} else if beforeRecord != nil {
			if userId := beforeRecord.Get("user"); userId != nil {
				auditRecord.Set("user_id", userId)
			} else if userId := beforeRecord.Get("created_by"); userId != nil {
				auditRecord.Set("user_id", userId)
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
				auditRecord.Set("before_changes", string(beforeJSON))
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
				auditRecord.Set("after_changes", string(afterJSON))
			} else {
				log.Printf("Failed to marshal after changes to JSON: %v", err)
			}
		} else {
			log.Printf("Failed to marshal after record data: %v", err)
		}
	}

	// Save the audit log
	if err := app.Save(auditRecord); err != nil {
		log.Printf("Failed to save audit log: %v", err)
		return err
	}

	log.Printf("Audit log created for %s event on %s record %s", 
		eventType, collectionName, recordId)

	return nil
}
