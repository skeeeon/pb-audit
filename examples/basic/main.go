package main

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/skeeeon/pb-audit" // Import using the GitHub repository path
)

func main() {
	// Initialize PocketBase
	app := pocketbase.New()
	
	// Setup audit logging with default options
	if err := pbaudit.Setup(app, pbaudit.DefaultOptions()); err != nil {
		log.Fatalf("Failed to setup audit logging: %v", err)
	}
	
	// Start the PocketBase app as usual
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
