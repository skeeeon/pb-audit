package pbaudit

// Options allows customizing the audit logging behavior.
type Options struct {
	// CollectionName is the name for audit logs collection
	CollectionName string

	// Event type logging toggles
	EnableStandardEvents bool // Create, update, delete operations
	EnableRequestEvents  bool // API request events
	EnableAuthEvents     bool // Authentication events

	// Custom event filter function (optional)
	// Return true to log the event, false to ignore
	// Allows filtering by collection and event type
	EventFilter func(collectionName, eventType string) bool

	// SchemaPath for optional collection import
	SchemaPath string

	// Whether to auto-create audit collection if it doesn't exist
	CreateAuditCollection bool

	// Whether to fail on schema import errors
	FailOnSchemaError bool

	// Log event details to console (only important events,
	// not all events to avoid excessive logging)
	LogToConsole bool
}

// DefaultOptions returns sensible defaults for Options.
func DefaultOptions() Options {
	return Options{
		CollectionName:        "audit_logs",
		EnableStandardEvents:  true,
		EnableRequestEvents:   true,
		EnableAuthEvents:      true,
		EventFilter:           nil, // No filter by default
		SchemaPath:            "",  // No schema by default
		CreateAuditCollection: true,
		FailOnSchemaError:     false,
		LogToConsole:          true,
	}
}

// applyDefaultOptions fills in default values for any missing options.
func applyDefaultOptions(options Options) Options {
	// Start with defaults
	defaults := DefaultOptions()

	// Apply user options, checking for empty values
	if options.CollectionName == "" {
		options.CollectionName = defaults.CollectionName
	}

	// For boolean fields we don't need to check - they'll be false by default which is fine
	// But we'll make sure the defaults if nothing is provided are the DefaultOptions values

	// Keep custom EventFilter if provided
	// If not provided, it should be nil which means no filtering

	return options
}
