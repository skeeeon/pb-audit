// Package pbaudit provides comprehensive audit logging for PocketBase applications.
//
// This library tracks all record operations (create, update, delete), API requests,
// and authentication events, creating a complete audit trail with before/after states,
// user attribution, and request metadata.
//
// DUAL-TRACKING SYSTEM:
// pb-audit uses a dual-tracking approach for complete audit trails:
//
// 1. REQUEST EVENTS (before commit):
//   - Capture user intent and request context (IP, user, method, URL)
//   - Include before state for updates/deletes
//   - Fire before database commit (may not complete if validation fails)
//
// 2. SUCCESS EVENTS (after commit):
//   - Confirm operation committed to database
//   - Include final state after all hooks/validations
//   - Guarantee operation succeeded
//
// This provides visibility into both what was attempted and what actually happened.
//
// Example usage:
//
//	app := pocketbase.New()
//	if err := pbaudit.Setup(app, pbaudit.DefaultOptions()); err != nil {
//	    log.Fatalf("Failed to setup audit logging: %v", err)
//	}
//	app.Start()
package pbaudit

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/skeeeon/pb-audit/internal/audit"
)

// Options configures the behavior of audit logging.
type Options struct {
	// Collection configuration
	CollectionName string // Name for audit logs collection (default: "audit_logs")

	// What to log
	LogRequestEvents bool // Log API request events (default: true)
	LogSuccessEvents bool // Log database success events (default: true)
	LogAuthEvents    bool // Log authentication events (default: true)

	// Optional filtering
	// EventFilter allows custom filtering logic for events
	// Return true to log the event, false to skip it
	// Parameters: collectionName, eventType
	//
	// Example:
	//   EventFilter: func(collectionName, eventType string) bool {
	//       // Only log events for sensitive collections
	//       return collectionName == "users" || collectionName == "payments"
	//   }
	EventFilter func(collectionName, eventType string) bool

	// Logging
	LogToConsole bool // Enable console logging (default: true)
}

// DefaultOptions returns sensible defaults for audit logging.
//
// Default configuration:
//   - CollectionName: "audit_logs"
//   - LogRequestEvents: true (track API operations)
//   - LogSuccessEvents: true (track database operations)
//   - LogAuthEvents: true (track authentication)
//   - EventFilter: nil (log all events)
//   - LogToConsole: true (enable logging)
func DefaultOptions() Options {
	return Options{
		CollectionName:   "audit_logs",
		LogRequestEvents: true,
		LogSuccessEvents: true,
		LogAuthEvents:    true,
		EventFilter:      nil,
		LogToConsole:     true,
	}
}

// Setup initializes audit logging for a PocketBase application.
//
// This is the main entry point that creates the audit collection and registers hooks.
//
// BEHAVIOR:
// - Non-destructive: Only creates collection if it doesn't exist
// - Preserves customizations: Won't overwrite API rules after initial setup
// - Always registers hooks: Even if collection already exists
//
// PARAMETERS:
//   - app: PocketBase application instance
//   - options: Configuration options (use DefaultOptions() for defaults)
//
// RETURNS:
//   - nil on successful setup
//   - error if setup fails
//
// Example:
//
//	app := pocketbase.New()
//
//	// With default options
//	if err := pbaudit.Setup(app, pbaudit.DefaultOptions()); err != nil {
//	    log.Fatal(err)
//	}
//
//	// With custom options
//	options := pbaudit.DefaultOptions()
//	options.CollectionName = "my_audit_logs"
//	options.LogAuthEvents = false
//	if err := pbaudit.Setup(app, options); err != nil {
//	    log.Fatal(err)
//	}
func Setup(app *pocketbase.PocketBase, options Options) error {
	// Apply defaults for any zero-value fields
	options = applyDefaults(options)

	// Validate options
	if err := validateOptions(options); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}

	// Convert public Options to internal Options
	internalOpts := audit.Options{
		CollectionName:   options.CollectionName,
		LogRequestEvents: options.LogRequestEvents,
		LogSuccessEvents: options.LogSuccessEvents,
		LogAuthEvents:    options.LogAuthEvents,
		EventFilter:      options.EventFilter,
		LogToConsole:     options.LogToConsole,
	}

	// Initialize after app bootstrap
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		// Wait for bootstrap to complete
		if err := e.Next(); err != nil {
			return err
		}

		return audit.Initialize(app, internalOpts)
	})

	return nil
}

// applyDefaults fills in default values for missing options.
func applyDefaults(options Options) Options {
	defaults := DefaultOptions()

	if options.CollectionName == "" {
		options.CollectionName = defaults.CollectionName
	}

	// For boolean fields, we can't distinguish between false and unset,
	// so we check if ALL logging options are false, which is unlikely to be intentional
	if !options.LogRequestEvents && !options.LogSuccessEvents && !options.LogAuthEvents {
		options.LogRequestEvents = defaults.LogRequestEvents
		options.LogSuccessEvents = defaults.LogSuccessEvents
		options.LogAuthEvents = defaults.LogAuthEvents
	}

	return options
}

// validateOptions validates the provided options.
func validateOptions(options Options) error {
	if options.CollectionName == "" {
		return fmt.Errorf("collection name cannot be empty")
	}

	// At least one logging option should be enabled
	if !options.LogRequestEvents && !options.LogSuccessEvents && !options.LogAuthEvents {
		return fmt.Errorf("at least one logging option must be enabled")
	}

	return nil
}

// Version is the library version.
const Version = "2.0.0"
