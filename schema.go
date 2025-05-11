package pbaudit

import (
	"fmt"
	"log"
	"os"

	"github.com/pocketbase/pocketbase"
)

// importCollectionsFromFile reads a schema file and imports all collections.
// It ensures the audit logs collection exists even after import.
func importCollectionsFromFile(app *pocketbase.PocketBase, schemaPath string, collectionName string) error {
	// Read the schema file
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	// Import collections using PocketBase's built-in functionality
	// Setting deleteMissing to false to prevent accidental data loss
	if err := app.ImportCollectionsByMarshaledJSON(schemaData, false); err != nil {
		return fmt.Errorf("failed to import collections: %w", err)
	}

	// Verify that the audit_logs collection exists even after successful import
	_, err = app.FindCollectionByNameOrId(collectionName)
	if err != nil {
		log.Printf("Audit logs collection '%s' not found in schema, creating it now...", collectionName)
		if err := ensureAuditCollection(app, collectionName); err != nil {
			return fmt.Errorf("failed to create audit logs collection: %w", err)
		}
	}

	log.Printf("Successfully imported collections from schema: %s", schemaPath)
	return nil
}
