// audit_collection.go - Creates the audit_logs collection for PocketBase
package main

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// setupAuditCollection ensures the audit_logs collection exists
func setupAuditCollection(app *pocketbase.PocketBase) error {
	// Check if collection already exists
	_, err := app.FindCollectionByNameOrId("audit_logs")
	if err == nil {
		log.Println("Audit logs collection already exists")
		return nil
	}

	// Create new base collection
	collection := core.NewBaseCollection("audit_logs")

	// Set security rules to limit access to admins only
	collection.ListRule = types.Pointer("@request.auth.type = 'admin'")
	collection.ViewRule = types.Pointer("@request.auth.type = 'admin'")
	collection.CreateRule = types.Pointer("@request.auth.type = 'admin'")
	collection.UpdateRule = types.Pointer("@request.auth.type = 'admin'")
	collection.DeleteRule = types.Pointer("@request.auth.type = 'admin'")

	// Add event_type select field with expanded options
	collection.Fields.Add(&core.SelectField{
		Name:     "event_type",
		Required: true,
		Values:   []string{"create", "update", "delete", "auth", "create_request", "update_request", "delete_request"},
	})

	// Add collection_name field
	collection.Fields.Add(&core.TextField{
		Name:     "collection_name",
		Required: true,
	})

	// Add record_id field
	collection.Fields.Add(&core.TextField{
		Name:     "record_id",
		Required: true,
	})

	// Add user_id field
	collection.Fields.Add(&core.TextField{
		Name:     "user_id",
		Required: false,
	})

	// Add auth_method field for auth events
	collection.Fields.Add(&core.TextField{
		Name:     "auth_method",
		Required: false,
	})

	// Add request_method field (GET, POST, PUT, DELETE, etc.)
	collection.Fields.Add(&core.TextField{
		Name:     "request_method",
		Required: false,
	})

	// Add request_ip field
	collection.Fields.Add(&core.TextField{
		Name:     "request_ip",
		Required: false,
	})
	
	// Add request_url field
	collection.Fields.Add(&core.TextField{
		Name:     "request_url",
		Required: false,
	})

	// Add timestamp field
	collection.Fields.Add(&core.DateField{
		Name:     "timestamp",
		Required: true,
	})

	// Add before_changes field (for storing original record data)
	collection.Fields.Add(&core.TextField{
		Name:     "before_changes",
		Required: false,
	})

	// Add after_changes field (for storing updated record data)
	collection.Fields.Add(&core.TextField{
		Name:     "after_changes",
		Required: false,
	})

	// Add timestamp fields
	collection.Fields.Add(&core.AutodateField{
		Name:     "created",
		OnCreate: true,
	})
	collection.Fields.Add(&core.AutodateField{
		Name:     "updated",
		OnCreate: true,
		OnUpdate: true,
	})

	// Create indexes for faster querying
	collection.AddIndex("idx_audit_collection_name", false, "collection_name", "")
	collection.AddIndex("idx_audit_record_id", false, "record_id", "")
	collection.AddIndex("idx_audit_timestamp", false, "timestamp", "")
	collection.AddIndex("idx_audit_user_id", false, "user_id", "")
	collection.AddIndex("idx_audit_event_type", false, "event_type", "")

	// Save the collection
	err = app.Save(collection)
	if err != nil {
		return err
	}

	log.Println("Successfully created audit_logs collection")
	return nil
}
