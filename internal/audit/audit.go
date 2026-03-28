package audit

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase"
)

// RetentionPolicy configures automatic cleanup of old audit logs.
type RetentionPolicy struct {
	MaxAge     time.Duration // Delete records older than this duration (0 = disabled)
	MaxRecords int           // Keep at most this many records (0 = disabled)
	Interval   string        // Cron expression for cleanup schedule
}

// Options holds configuration for audit logging setup.
type Options struct {
	// Collection configuration
	CollectionName string // Name for the audit logs collection (default: "audit_logs")

	// What to log
	LogRequestEvents bool // Log API request events (default: true)
	LogSuccessEvents bool // Log database success events (default: true)
	LogAuthEvents    bool // Log authentication events (default: true)

	// Optional filtering
	// EventFilter allows custom filtering logic for events
	// Return true to log the event, false to skip it
	// Parameters: collectionName, eventType
	EventFilter func(collectionName, eventType string) bool

	// Retention policy for automatic cleanup (nil = no cleanup)
	Retention *RetentionPolicy

	// Logging
	LogToConsole bool // Enable console logging of audit events (default: true)
}

// Initialize sets up audit logging in the correct order.
//
// SETUP PROCESS:
// 1. Check if audit logs collection exists
// 2. Create collection if needed (non-destructive)
// 3. Register hooks for tracking operations
//
// NON-DESTRUCTIVE BEHAVIOR:
// - Only creates collection if it doesn't exist
// - Sets API rules only on initial creation
// - Preserves any customizations made after setup
// - Always registers hooks (even if collection exists)
//
// PARAMETERS:
//   - app: PocketBase application instance
//   - options: Configuration options
//
// RETURNS:
//   - nil on successful setup
//   - error if setup fails
func Initialize(app *pocketbase.PocketBase, options Options) error {
	if options.LogToConsole {
		fmt.Println("🚀 START Initializing PocketBase audit logging...")
	}

	// Check if audit logs collection exists
	_, err := app.FindCollectionByNameOrId(options.CollectionName)
	isFirstTimeSetup := err != nil

	if isFirstTimeSetup {
		if options.LogToConsole {
			fmt.Println("ℹ️  INFO   Performing first-time setup for audit logs collection...")
		}

		// Create audit logs collection
		if err := ensureAuditCollection(app, options.CollectionName); err != nil {
			return fmt.Errorf("failed to create audit logs collection: %w", err)
		}

		if options.LogToConsole {
			fmt.Println("✅ SUCCESS Audit logs collection created")
		}
	} else {
		if options.LogToConsole {
			fmt.Println("ℹ️  INFO   Audit logs collection already exists. Skipping schema modifications.")
		}
	}

	// Register hooks for automatic audit logging (always do this)
	if err := registerHooks(app, options); err != nil {
		return fmt.Errorf("failed to register audit hooks: %w", err)
	}

	// Register retention policy if configured
	if options.Retention != nil {
		if err := registerRetention(app, options); err != nil {
			return fmt.Errorf("failed to register retention policy: %w", err)
		}
	}

	if options.LogToConsole {
		fmt.Println("✅ SUCCESS PocketBase audit logging initialized successfully")
		fmt.Printf("ℹ️  INFO   - Collection: %s\n", options.CollectionName)
		fmt.Printf("ℹ️  INFO   - Log request events: %v\n", options.LogRequestEvents)
		fmt.Printf("ℹ️  INFO   - Log success events: %v\n", options.LogSuccessEvents)
		fmt.Printf("ℹ️  INFO   - Log auth events: %v\n", options.LogAuthEvents)
		if options.Retention != nil {
			fmt.Printf("ℹ️  INFO   - Retention: maxAge=%v, maxRecords=%d, interval=%s\n",
				options.Retention.MaxAge, options.Retention.MaxRecords, options.Retention.Interval)
		}
	}

	return nil
}
