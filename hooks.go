package pbaudit

import (
	"log"
	"reflect"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// setupStandardEventHooks registers hooks for standard database operations
func (l *logger) setupStandardEventHooks() {
	// Register hooks for record creation events
	l.app.OnRecordAfterCreateSuccess().BindFunc(func(e *core.RecordEvent) error {
		// Get the collection name from the record
		collectionName := e.Record.Collection().Name
		
		// Skip audit logs collection to prevent recursion (handled in logEvent, but checking here saves processing)
		if collectionName == l.options.CollectionName {
			return e.Next()
		}
		
		// For create events, there's no "before" state
		return l.logEvent(e.Record, nil, collectionName, EventTypeCreate, nil)
	})

	// Register hooks for record update events
	l.app.OnRecordAfterUpdateSuccess().BindFunc(func(e *core.RecordEvent) error {
		// Get the collection name from the record
		collectionName := e.Record.Collection().Name
		
		// Skip audit logs collection to prevent recursion
		if collectionName == l.options.CollectionName {
			return e.Next()
		}
		
		// For updates through standard events, we don't have easy access to the previous state
		return l.logEvent(e.Record, nil, collectionName, EventTypeUpdate, nil)
	})

	// Register hooks for record deletion events
	l.app.OnRecordAfterDeleteSuccess().BindFunc(func(e *core.RecordEvent) error {
		// Get the collection name from the record
		collectionName := e.Record.Collection().Name
		
		// Skip audit logs collection to prevent recursion
		if collectionName == l.options.CollectionName {
			return e.Next()
		}
		
		// For delete events, the "after" state doesn't exist, but we have the "before" state
		return l.logEvent(nil, e.Record, collectionName, EventTypeDelete, nil)
	})
	
	log.Println("PocketBase audit: Standard event hooks registered")
}

// extractIP attempts to extract the client IP using various methods available in PocketBase
// It attempts multiple approaches for better compatibility across different PocketBase versions
func extractIP(e interface{}) string {
	// First try direct RealIP() method if available (type assertion)
	if reqEvent, ok := e.(interface{ RealIP() string }); ok {
		return reqEvent.RealIP()
	}
	
	// Try to get RequestInfo
	var reqInfo *core.RequestInfo
	var err error
	
	// Type assertion to get RequestInfo
	if hasRequestInfo, ok := e.(interface{ RequestInfo() (*core.RequestInfo, error) }); ok {
		reqInfo, err = hasRequestInfo.RequestInfo()
		if err != nil {
			log.Printf("Failed to get request info: %v", err)
			return "unknown"
		}
	} else {
		return "unknown"
	}
	
	// Now parse headers from RequestInfo
	
	// Try common headers (case insensitive)
	headerMap := make(map[string]string)
	for k, v := range reqInfo.Headers {
		headerMap[strings.ToLower(k)] = v
	}
	
	// Try Cloudflare
	if cfIP, ok := headerMap["cf-connecting-ip"]; ok && cfIP != "" {
		return cfIP
	}
	
	// X-Forwarded-For - first IP is usually the client
	if forwardedFor, ok := headerMap["x-forwarded-for"]; ok && forwardedFor != "" {
		ips := strings.Split(forwardedFor, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	
	// X-Real-IP
	if realIP, ok := headerMap["x-real-ip"]; ok && realIP != "" {
		return realIP
	}
	
	// Fly.io
	if flyIP, ok := headerMap["fly-client-ip"]; ok && flyIP != "" {
		return flyIP
	}
	
	return "unknown"
}

// setupRequestEventHooks registers hooks for API request events
func (l *logger) setupRequestEventHooks() {
	// Register hooks for record create request events
	l.app.OnRecordCreateRequest().BindFunc(func(e *core.RecordRequestEvent) error {
		// Skip audit logs collection to prevent recursion
		if e.Collection.Name == l.options.CollectionName {
			return e.Next()
		}
		
		// Extract request information
		requestInfo := make(map[string]interface{})
		
		// Use helper function to extract IP
		requestInfo[AuditLogFields.RequestIP] = extractIP(e)
		
		// Use RequestInfo method to get additional request details
		reqInfo, err := e.RequestInfo()
		if err != nil {
			log.Printf("Failed to get request info: %v", err)
		} else {
			requestInfo[AuditLogFields.RequestMethod] = reqInfo.Method
			requestInfo[AuditLogFields.RequestURL] = reqInfo.Context
			
			// Add authenticated user if available
			if reqInfo.Auth != nil {
				requestInfo[AuditLogFields.UserID] = reqInfo.Auth.Id
			}
		}
		
		// For create requests, there's no "before" state
		err = l.logEvent(e.Record, nil, e.Collection.Name, EventTypeCreateReq, requestInfo)
		if err != nil {
			log.Printf("Failed to log create request event: %v", err)
		}
		
		return e.Next()
	})
	
	// Register hooks for record update request events
	l.app.OnRecordUpdateRequest().BindFunc(func(e *core.RecordRequestEvent) error {
		// Skip audit logs collection to prevent recursion
		if e.Collection.Name == l.options.CollectionName {
			return e.Next()
		}
		
		// Load the original record from the database to get the "before" state
		originalRecord, err := l.app.FindRecordById(e.Collection.Name, e.Record.Id)
		if err != nil {
			log.Printf("Failed to load original record for update tracking: %v", err)
		}
		
		// Extract request information
		requestInfo := make(map[string]interface{})
		
		// Use helper function to extract IP
		requestInfo[AuditLogFields.RequestIP] = extractIP(e)
		
		// Use RequestInfo method to get additional request details
		reqInfo, err := e.RequestInfo()
		if err != nil {
			log.Printf("Failed to get request info: %v", err)
		} else {
			requestInfo[AuditLogFields.RequestMethod] = reqInfo.Method
			requestInfo[AuditLogFields.RequestURL] = reqInfo.Context
			
			// Add authenticated user if available
			if reqInfo.Auth != nil {
				requestInfo[AuditLogFields.UserID] = reqInfo.Auth.Id
			}
		}
		
		// Pass both original and updated record
		err = l.logEvent(e.Record, originalRecord, e.Collection.Name, EventTypeUpdateReq, requestInfo)
		if err != nil {
			log.Printf("Failed to log update request event: %v", err)
		}
		
		return e.Next()
	})
	
	// Register hooks for record delete request events
	l.app.OnRecordDeleteRequest().BindFunc(func(e *core.RecordRequestEvent) error {
		// Skip audit logs collection to prevent recursion
		if e.Collection.Name == l.options.CollectionName {
			return e.Next()
		}
		
		// Extract request information
		requestInfo := make(map[string]interface{})
		
		// Use helper function to extract IP
		requestInfo[AuditLogFields.RequestIP] = extractIP(e)
		
		// Use RequestInfo method to get additional request details
		reqInfo, err := e.RequestInfo()
		if err != nil {
			log.Printf("Failed to get request info: %v", err)
		} else {
			requestInfo[AuditLogFields.RequestMethod] = reqInfo.Method
			requestInfo[AuditLogFields.RequestURL] = reqInfo.Context
			
			// Add authenticated user if available
			if reqInfo.Auth != nil {
				requestInfo[AuditLogFields.UserID] = reqInfo.Auth.Id
			}
		}
		
		// For delete operations, the "after" state doesn't exist, but we have the "before" state
		err = l.logEvent(nil, e.Record, e.Collection.Name, EventTypeDeleteReq, requestInfo)
		if err != nil {
			log.Printf("Failed to log delete request event: %v", err)
		}
		
		return e.Next()
	})
	
	log.Println("PocketBase audit: Request event hooks registered")
}

// setupAuthEventHooks registers hooks for authentication events
func (l *logger) setupAuthEventHooks() {
	l.app.OnRecordAuthRequest().BindFunc(func(e *core.RecordAuthRequestEvent) error {
		// Extract request information
		requestInfo := make(map[string]interface{})
		
		// Add auth method
		requestInfo[AuditLogFields.AuthMethod] = e.AuthMethod
		
		// Add user ID if record exists
		if e.Record != nil {
			requestInfo[AuditLogFields.UserID] = e.Record.Id
		}
		
		// Use helper function to extract IP
		requestInfo[AuditLogFields.RequestIP] = extractIP(e)
		
		// Extract additional request data
		reqInfo, err := e.RequestInfo()
		if err != nil {
			log.Printf("Failed to get request info: %v", err)
		} else {
			requestInfo[AuditLogFields.RequestMethod] = reqInfo.Method
			requestInfo[AuditLogFields.RequestURL] = reqInfo.Context
		}
		
		// For auth events, there's no "before" state but we still have the current state
		err = l.logEvent(e.Record, nil, e.Record.Collection().Name, EventTypeAuth, requestInfo)
		if err != nil {
			log.Printf("Failed to log auth event: %v", err)
		}
		
		return e.Next()
	})
	
	log.Println("PocketBase audit: Auth event hooks registered")
}
