package pbaudit

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

// ensureAuditCollection creates the audit_logs collection if it doesn't exist.
// This is called during Setup() if CreateAuditCollection option is true.
func ensureAuditCollection(app *pocketbase.PocketBase, collectionName string) error {
	// Check if collection already exists
	_, err := app.FindCollectionByNameOrId(collectionName)
	if err == nil {
		log.Printf("Audit logs collection '%s' already exists", collectionName)
		return nil
	}

	// Create new base collection
	collection := core.NewBaseCollection(collectionName)

	// Set security rules to limit access to admins only
	collection.ListRule = types.Pointer("@request.auth.type = 'admin'")
	collection.ViewRule = types.Pointer("@request.auth.type = 'admin'")
	collection.CreateRule = types.Pointer("@request.auth.type = 'admin'")
	collection.UpdateRule = types.Pointer("@request.auth.type = 'admin'")
	collection.DeleteRule = types.Pointer("@request.auth.type = 'admin'")

	// Add event_type select field with expanded options
	collection.Fields.Add(&core.SelectField{
		Name:     AuditLogFields.EventType,
		Required: true,
		Values:   AllEventTypes,
	})

	// Add collection_name field
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.CollectionName,
		Required: true,
	})

	// Add record_id field
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.RecordID,
		Required: true,
	})

	// Add user_id field
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.UserID,
		Required: false,
	})

	// Add auth_method field for auth events
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.AuthMethod,
		Required: false,
	})

	// Add request_method field (GET, POST, PUT, DELETE, etc.)
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.RequestMethod,
		Required: false,
	})

	// Add request_ip field
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.RequestIP,
		Required: false,
	})
	
	// Add request_url field
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.RequestURL,
		Required: false,
	})

	// Add timestamp field
	collection.Fields.Add(&core.DateField{
		Name:     AuditLogFields.Timestamp,
		Required: true,
	})

	// Add before_changes field (for storing original record data)
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.BeforeChanges,
		Required: false,
	})

	// Add after_changes field (for storing updated record data)
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.AfterChanges,
		Required: false,
	})

	// Add timestamp fields
	collection.Fields.Add(&core.AutodateField{
		Name:     AuditLogFields.Created,
		OnCreate: true,
	})
	collection.Fields.Add(&core.AutodateField{
		Name:     AuditLogFields.Updated,
		OnCreate: true,
		OnUpdate: true,
	})

	// Create indexes for faster querying
	collection.AddIndex("idx_audit_collection_name", false, AuditLogFields.CollectionName, "")
	collection.AddIndex("idx_audit_record_id", false, AuditLogFields.RecordID, "")
	collection.AddIndex("idx_audit_timestamp", false, AuditLogFields.Timestamp, "")
	collection.AddIndex("idx_audit_user_id", false, AuditLogFields.UserID, "")
	collection.AddIndex("idx_audit_event_type", false, AuditLogFields.EventType, "")

	// Save the collection
	err = app.Save(collection)
	if err != nil {
		return err
	}

	log.Printf("Successfully created audit logs collection: %s", collectionName)
	return nil
}
