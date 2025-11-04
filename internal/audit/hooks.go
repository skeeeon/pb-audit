package audit

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// registerHooks sets up all audit logging hooks.
//
// HOOK TYPES:
// 1. Request hooks - Capture API operations before commit with full context
// 2. Success hooks - Confirm database operations after successful commit
// 3. Auth hooks - Track authentication events
//
// The dual-tracking system (request + success) provides complete audit trail:
// - Request events show user intent with IP, method, and before state
// - Success events confirm the operation completed and show final state
//
// PARAMETERS:
//   - app: PocketBase application instance
//   - options: Configuration options
//
// RETURNS:
//   - nil on successful hook registration
//   - error if registration fails
func registerHooks(app *pocketbase.PocketBase, options Options) error {
	logger := newLogger(app, options)

	// Register request hooks (API operations before commit)
	if options.LogRequestEvents {
		if err := registerRequestHooks(app, logger); err != nil {
			return err
		}
		if options.LogToConsole {
			fmt.Println("✅ SUCCESS Request event hooks registered")
		}
	}

	// Register success hooks (database operations after commit)
	if options.LogSuccessEvents {
		if err := registerSuccessHooks(app, logger); err != nil {
			return err
		}
		if options.LogToConsole {
			fmt.Println("✅ SUCCESS Success event hooks registered")
		}
	}

	// Register auth hooks (authentication events)
	if options.LogAuthEvents {
		if err := registerAuthHooks(app, logger); err != nil {
			return err
		}
		if options.LogToConsole {
			fmt.Println("✅ SUCCESS Auth event hooks registered")
		}
	}

	return nil
}

// registerRequestHooks registers hooks for API request events.
//
// These hooks capture:
// - User-initiated operations via API
// - Request metadata (IP, method, URL)
// - Before state for updates and deletes
// - After state for creates and updates
//
// Request events fire BEFORE the database commit, so they may not succeed
// if validation fails. The corresponding success event confirms completion.
func registerRequestHooks(app *pocketbase.PocketBase, logger *logger) error {
	// Hook: Create Request
	app.OnRecordCreateRequest().BindFunc(func(e *core.RecordRequestEvent) error {
		// Skip audit logs collection to prevent recursion
		if e.Collection.Name == logger.options.CollectionName {
			return e.Next()
		}

		// Extract request information
		requestInfo := extractRequestInfo(e)

		// For create requests, there's no before state
		if err := logger.logEvent(e.Record, nil, e.Collection.Name, EventTypeCreateRequest, requestInfo); err != nil {
			if logger.options.LogToConsole {
				fmt.Printf("⚠️  WARNING Failed to log create request: %v\n", err)
			}
		}

		return e.Next()
	})

	// Hook: Update Request
	app.OnRecordUpdateRequest().BindFunc(func(e *core.RecordRequestEvent) error {
		// Skip audit logs collection to prevent recursion
		if e.Collection.Name == logger.options.CollectionName {
			return e.Next()
		}

		// Load original record to get before state
		originalRecord, err := logger.app.FindRecordById(e.Collection.Name, e.Record.Id)
		if err != nil {
			if logger.options.LogToConsole {
				fmt.Printf("⚠️  WARNING Failed to load original record for update: %v\n", err)
			}
			originalRecord = nil
		}

		// Extract request information
		requestInfo := extractRequestInfo(e)

		// Log with both before and after states
		if err := logger.logEvent(e.Record, originalRecord, e.Collection.Name, EventTypeUpdateRequest, requestInfo); err != nil {
			if logger.options.LogToConsole {
				fmt.Printf("⚠️  WARNING Failed to log update request: %v\n", err)
			}
		}

		return e.Next()
	})

	// Hook: Delete Request
	app.OnRecordDeleteRequest().BindFunc(func(e *core.RecordRequestEvent) error {
		// Skip audit logs collection to prevent recursion
		if e.Collection.Name == logger.options.CollectionName {
			return e.Next()
		}

		// Extract request information
		requestInfo := extractRequestInfo(e)

		// For delete requests, record is the before state, no after state
		if err := logger.logEvent(nil, e.Record, e.Collection.Name, EventTypeDeleteRequest, requestInfo); err != nil {
			if logger.options.LogToConsole {
				fmt.Printf("⚠️  WARNING Failed to log delete request: %v\n", err)
			}
		}

		return e.Next()
	})

	return nil
}

// registerSuccessHooks registers hooks for successful database operations.
//
// These hooks capture:
// - All database operations (API and programmatic)
// - Final state after all validations and hooks
// - Confirmation that operation committed successfully
//
// Success events fire AFTER the database commit, guaranteeing the operation
// completed. However, they have limited access to request metadata.
func registerSuccessHooks(app *pocketbase.PocketBase, logger *logger) error {
	// Hook: Create Success
	app.OnRecordAfterCreateSuccess().BindFunc(func(e *core.RecordEvent) error {
		collectionName := e.Record.Collection().Name

		// Skip audit logs collection to prevent recursion
		if collectionName == logger.options.CollectionName {
			return e.Next()
		}

		// For create events, there's no before state
		if err := logger.logEvent(e.Record, nil, collectionName, EventTypeCreate, nil); err != nil {
			if logger.options.LogToConsole {
				fmt.Printf("⚠️  WARNING Failed to log create success: %v\n", err)
			}
		}

		return e.Next()
	})

	// Hook: Update Success
	app.OnRecordAfterUpdateSuccess().BindFunc(func(e *core.RecordEvent) error {
		collectionName := e.Record.Collection().Name

		// Skip audit logs collection to prevent recursion
		if collectionName == logger.options.CollectionName {
			return e.Next()
		}

		// For update success events, we don't have easy access to before state
		// The request hook already captured it, this confirms the commit
		if err := logger.logEvent(e.Record, nil, collectionName, EventTypeUpdate, nil); err != nil {
			if logger.options.LogToConsole {
				fmt.Printf("⚠️  WARNING Failed to log update success: %v\n", err)
			}
		}

		return e.Next()
	})

	// Hook: Delete Success
	app.OnRecordAfterDeleteSuccess().BindFunc(func(e *core.RecordEvent) error {
		collectionName := e.Record.Collection().Name

		// Skip audit logs collection to prevent recursion
		if collectionName == logger.options.CollectionName {
			return e.Next()
		}

		// For delete events, record is the before state, no after state
		if err := logger.logEvent(nil, e.Record, collectionName, EventTypeDelete, nil); err != nil {
			if logger.options.LogToConsole {
				fmt.Printf("⚠️  WARNING Failed to log delete success: %v\n", err)
			}
		}

		return e.Next()
	})

	return nil
}

// registerAuthHooks registers hooks for authentication events.
//
// These hooks capture:
// - User login events
// - Authentication method used
// - Request metadata (IP, etc.)
//
// NOTE: Auth events are only logged for regular user authentication.
// Admin/superuser authentication is not logged since admins are not
// in the users collection.
func registerAuthHooks(app *pocketbase.PocketBase, logger *logger) error {
	app.OnRecordAuthRequest().BindFunc(func(e *core.RecordAuthRequestEvent) error {
		if e.Record == nil {
			return e.Next()
		}

		// Only log auth events for users in the users collection
		// Skip admin/superuser authentication
		if e.Record.Collection().Name != "users" {
			return e.Next()
		}

		// Extract request information
		requestInfo := make(map[string]interface{})

		// Add auth method
		requestInfo[AuditLogFields.AuthMethod] = e.AuthMethod

		// Add user ID
		requestInfo[AuditLogFields.User] = e.Record.Id

		// Extract IP and other request details
		reqInfo, err := e.RequestInfo()
		if err == nil {
			requestInfo[AuditLogFields.RequestIP] = extractClientIP(reqInfo)
			requestInfo[AuditLogFields.RequestMethod] = reqInfo.Method
			requestInfo[AuditLogFields.RequestURL] = reqInfo.Context
		}

		// Log auth event with current user state
		if err := logger.logEvent(e.Record, nil, e.Record.Collection().Name, EventTypeAuth, requestInfo); err != nil {
			if logger.options.LogToConsole {
				fmt.Printf("⚠️  WARNING Failed to log auth event: %v\n", err)
			}
		}

		return e.Next()
	})

	return nil
}

// extractRequestInfo extracts common request information from record request events.
//
// EXTRACTED DATA:
// - Client IP address (via extractClientIP)
// - HTTP method (GET, POST, PUT, DELETE, etc.)
// - Request URL path
// - Authenticated user ID (if available)
//
// PARAMETERS:
//   - e: Record request event
//
// RETURNS:
//   - Map of request metadata
func extractRequestInfo(e *core.RecordRequestEvent) map[string]interface{} {
	requestInfo := make(map[string]interface{})

	reqInfo, err := e.RequestInfo()
	if err != nil {
		return requestInfo
	}

	// Extract IP address
	requestInfo[AuditLogFields.RequestIP] = extractClientIP(reqInfo)

	// Extract HTTP method
	requestInfo[AuditLogFields.RequestMethod] = reqInfo.Method

	// Extract request URL
	requestInfo[AuditLogFields.RequestURL] = reqInfo.Context

	// Extract authenticated user if available
	if reqInfo.Auth != nil {
		requestInfo[AuditLogFields.User] = reqInfo.Auth.Id
	}

	return requestInfo
}
