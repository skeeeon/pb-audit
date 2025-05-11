// Package pbaudit provides comprehensive audit logging capabilities for PocketBase applications.
// It tracks record operations (create, update, delete), API requests, and authentication events.
package pbaudit

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Setup initializes audit logging for a PocketBase instance.
// This is the main entry point for the library.
//
// Example usage:
//
//	app := pocketbase.New()
//	if err := pbaudit.Setup(app, pbaudit.DefaultOptions()); err != nil {
//	    log.Fatalf("Failed to setup audit logging: %v", err)
//	}
//	app.Start()
func Setup(app *pocketbase.PocketBase, options Options) error {
	// Validate and apply default options
	options = applyDefaultOptions(options)

	// Register the bootstrap hook to ensure collection setup happens after PocketBase is ready
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		// Wait for bootstrap to complete before accessing the database
		if err := e.Next(); err != nil {
			return err
		}

		// Create audit collection if needed
		if options.CreateAuditCollection {
			if err := ensureAuditCollection(app, options.CollectionName); err != nil {
				log.Printf("Warning: Failed to setup audit logs collection: %v", err)
				return err
			}
		}

		// Initialize schema if path provided
		if options.SchemaPath != "" {
			if err := importCollectionsFromFile(app, options.SchemaPath, options.CollectionName); err != nil {
				// Just log warning, don't return error unless configured to fail
				log.Printf("Warning: Failed to import collections from schema: %v", err)
				if options.FailOnSchemaError {
					return err
				}
			}
		}

		// Create logger instance
		logger := newLogger(app, options)

		// Register hooks based on options
		if options.EnableStandardEvents {
			logger.setupStandardEventHooks()
		}

		if options.EnableRequestEvents {
			logger.setupRequestEventHooks()
		}

		if options.EnableAuthEvents {
			logger.setupAuthEventHooks()
		}

		log.Printf("PocketBase audit logging initialized successfully (collection: %s)", options.CollectionName)
		return nil
	})

	return nil
// Package pbaudit provides comprehensive audit logging capabilities for PocketBase applications.
// It tracks record operations (create, update, delete), API requests, and authentication events.
package pbaudit

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Setup initializes audit logging for a PocketBase instance.
// This is the main entry point for the library.
//
// Example usage:
//
//	app := pocketbase.New()
//	if err := pbaudit.Setup(app, pbaudit.DefaultOptions()); err != nil {
//	    log.Fatalf("Failed to setup audit logging: %v", err)
//	}
//	app.Start()
func Setup(app *pocketbase.PocketBase, options Options) error {
	// Validate and apply default options
	options = applyDefaultOptions(options)

	// Register the bootstrap hook to ensure collection setup happens after PocketBase is ready
	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		// Wait for bootstrap to complete before accessing the database
		if err := e.Next(); err != nil {
			return err
		}

		// Create audit collection if needed
		if options.CreateAuditCollection {
			if err := ensureAuditCollection(app, options.CollectionName); err != nil {
				log.Printf("Warning: Failed to setup audit logs collection: %v", err)
				return err
			}
		}

		// Initialize schema if path provided
		if options.SchemaPath != "" {
			if err := importCollectionsFromFile(app, options.SchemaPath, options.CollectionName); err != nil {
				// Just log warning, don't return error unless configured to fail
				log.Printf("Warning: Failed to import collections from schema: %v", err)
				if options.FailOnSchemaError {
					return err
				}
			}
		}

		// Create logger instance
		logger := newLogger(app, options)

		// Register hooks based on options
		if options.EnableStandardEvents {
			logger.setupStandardEventHooks()
		}

		if options.EnableRequestEvents {
			logger.setupRequestEventHooks()
		}

		if options.EnableAuthEvents {
			logger.setupAuthEventHooks()
		}

		log.Printf("PocketBase audit logging initialized successfully (collection: %s)", options.CollectionName)
		return nil
	})

	return nil
}}
