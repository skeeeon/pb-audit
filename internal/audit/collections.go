package audit

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// ensureAuditCollection creates the audit logs collection if it doesn't exist.
//
// BEHAVIOR:
// - Non-destructive: Only creates collection if it doesn't exist
// - Sets API rules only on initial creation
// - Preserves any customizations made after initial setup
//
// COLLECTION SCHEMA:
// - event_type: Select field with all event types
// - collection_name: Text field for collection name
// - record_id: Text field for record ID
// - user: Relation to users collection (preserved on user delete)
// - auth_method: Text field for authentication method
// - request_method: Text field for HTTP method
// - request_ip: Text field for client IP
// - request_url: Text field for request path
// - timestamp: Date field for event time
// - changed_fields: JSON array naming the fields that differ
// - before_changes: JSON field for record state before operation (opt-in)
// - after_changes: JSON field for record state after operation (opt-in)
//
// PARAMETERS:
//   - app: PocketBase application instance
//   - collectionName: Name for the audit logs collection
//
// RETURNS:
//   - nil on success
//   - error if collection creation fails
func ensureAuditCollection(app *pocketbase.PocketBase, collectionName string) error {
	// Check if collection already exists (non-destructive)
	_, err := app.FindCollectionByNameOrId(collectionName)
	if err == nil {
		// Collection exists, preserve any customizations
		return nil
	}

	// Create new base collection
	collection := core.NewBaseCollection(collectionName)

	// Add event_type select field
	collection.Fields.Add(&core.SelectField{
		Name:      AuditLogFields.EventType,
		Required:  true,
		MaxSelect: 1,
		Values:    AllEventTypes,
	})

	// Add collection_name field
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.CollectionName,
		Required: true,
		Max:      255,
	})

	// Add record_id field (not required - create_request events don't have IDs yet)
	collection.Fields.Add(&core.TextField{
		Name:     AuditLogFields.RecordID,
		Required: false,
		Max:      255,
	})

	// Get users collection for relation field
	usersCollection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return fmt.Errorf("users collection not found: %w", err)
	}

	// Add user relation field (not required - admins aren't in users collection)
	// CascadeDelete is false to preserve audit logs even if user is deleted
	collection.Fields.Add(&core.RelationField{
		Name:          AuditLogFields.User,
		Required:      false,
		MaxSelect:     1,
		CollectionId:  usersCollection.Id,
		CascadeDelete: false,
	})

	// Add auth_method field for authentication events
	collection.Fields.Add(&core.TextField{
		Name: AuditLogFields.AuthMethod,
		Max:  100,
	})

	// Add request_method field (GET, POST, PUT, DELETE, etc.)
	collection.Fields.Add(&core.TextField{
		Name: AuditLogFields.RequestMethod,
		Max:  20,
	})

	// Add request_ip field for client IP address
	collection.Fields.Add(&core.TextField{
		Name: AuditLogFields.RequestIP,
		Max:  100,
	})

	// Add request_url field for request path
	collection.Fields.Add(&core.TextField{
		Name: AuditLogFields.RequestURL,
		Max:  2000,
	})

	// Add timestamp field
	collection.Fields.Add(&core.DateField{
		Name:     AuditLogFields.Timestamp,
		Required: true,
	})

	// Add changed_fields JSON array naming the fields that moved.
	//
	// Written for every record event, whether or not the collection opted into
	// value snapshots -- it is the part of the trail that is always safe to
	// keep. See changedFields in diff.go. Small by construction: a list of
	// field names, never their contents.
	collection.Fields.Add(&core.JSONField{
		Name:    AuditLogFields.ChangedFields,
		MaxSize: 100000,
	})

	// Add before_changes JSON field for storing record state before operation.
	// Only populated for collections named in Options.SnapshotCollections.
	collection.Fields.Add(&core.JSONField{
		Name:    AuditLogFields.BeforeChanges,
		MaxSize: 2000000, // 2MB limit
	})

	// Add after_changes JSON field for storing record state after operation.
	// Only populated for collections named in Options.SnapshotCollections.
	collection.Fields.Add(&core.JSONField{
		Name:    AuditLogFields.AfterChanges,
		MaxSize: 2000000, // 2MB limit
	})

	// Add auto-generated timestamp fields
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
	collection.Indexes = []string{
		fmt.Sprintf("CREATE INDEX idx_audit_collection_name ON %s (%s)", collectionName, AuditLogFields.CollectionName),
		fmt.Sprintf("CREATE INDEX idx_audit_record_id ON %s (%s)", collectionName, AuditLogFields.RecordID),
		fmt.Sprintf("CREATE INDEX idx_audit_timestamp ON %s (%s)", collectionName, AuditLogFields.Timestamp),
		fmt.Sprintf("CREATE INDEX idx_audit_user ON %s (%s)", collectionName, AuditLogFields.User),
		fmt.Sprintf("CREATE INDEX idx_audit_event_type ON %s (%s)", collectionName, AuditLogFields.EventType),
		// Composite indexes for common queries
		fmt.Sprintf("CREATE INDEX idx_audit_collection_timestamp ON %s (%s, %s)", collectionName, AuditLogFields.CollectionName, AuditLogFields.Timestamp),
		fmt.Sprintf("CREATE INDEX idx_audit_user_timestamp ON %s (%s, %s)", collectionName, AuditLogFields.User, AuditLogFields.Timestamp),
	}

	// API rules are left nil, which in PocketBase means superusers only. Set
	// only on initial creation; you can change them afterwards without pb-audit
	// overwriting them.
	//
	// These used to be `@request.auth.type = 'admin'`, which is PocketBase
	// v0.22 syntax: superusers moved into their own `_superusers` auth
	// collection in v0.23 and `@request.auth.type` stopped existing. A nil rule
	// says the same thing, says it in the form the framework itself uses, and
	// cannot go stale the next time the auth model moves.
	//
	// Superuser-only is the right default for a collection that exists to be
	// evidence: an audit trail an actor can edit is not one, and the rows carry
	// whatever the audited collections carry. Widen it deliberately, and read
	// the note on SnapshotCollections first -- a rule that lets someone read
	// their own audit rows also lets them read every value those rows hold.

	// Save the collection
	if err := app.Save(collection); err != nil {
		return fmt.Errorf("failed to create audit logs collection: %w", err)
	}

	return nil
}
