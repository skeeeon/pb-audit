package main

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/skeeeon/pb-audit"
)

func main() {
	// Initialize PocketBase
	app := pocketbase.New()

	// Setup audit logging with default options
	if err := pbaudit.Setup(app, pbaudit.DefaultOptions()); err != nil {
		log.Fatalf("Failed to setup audit logging: %v", err)
	}

	// Example: Custom configuration
	// options := pbaudit.DefaultOptions()
	// options.CollectionName = "my_audit_logs"
	// options.LogAuthEvents = false
	// options.EventFilter = func(collectionName, eventType string) bool {
	//     // Only log events for sensitive collections
	//     return collectionName == "users" || collectionName == "payments"
	// }
	// if err := pbaudit.Setup(app, options); err != nil {
	//     log.Fatalf("Failed to setup audit logging: %v", err)
	// }

	// Start the PocketBase app as usual
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
