package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	"github.com/skeeeon/pb-audit"
)

func main() {
	// Initialize PocketBase
	app := pocketbase.New()

	// Add the PocketBase migrate command
	migratecmd.MustRegister(app, app.RootCmd, &migratecmd.Options{
		Automigrate: true, // auto create migration files when collections change
	})

	// Setup audit logging with default options
	if err := pbaudit.Setup(app, pbaudit.DefaultOptions()); err != nil {
		log.Fatalf("Failed to setup audit logging: %v", err)
	}

	// Or with custom options
	/*
	options := pbaudit.DefaultOptions()
	options.CollectionName = "my_audit_logs"
	options.EnableAuthEvents = false // Disable auth event logging
	options.EventFilter = func(collectionName, eventType string) bool {
		// Only log events for specific collections
		return collectionName == "users" || collectionName == "sensitive_records"
	}
	options.SchemaPath = filepath.Join(os.Getenv("PB_SCHEMA_PATH"), "pb_schema.json")
	
	if err := pbaudit.Setup(app, options); err != nil {
		log.Fatalf("Failed to setup audit logging: %v", err)
	}
	*/

	// You can add custom routes or other configuration here
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/api/hello", func(c echo.Context) error {
			return c.String(200, "Hello world!")
		})
		return nil
	})

	// Start the PocketBase app as usual
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
