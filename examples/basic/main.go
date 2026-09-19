package main

import (
	"log"

	"github.com/pocketbase/pocketbase"
	pbaudit "github.com/skeeeon/pb-audit"
)

func main() {
	app := pocketbase.New()

	options := pbaudit.DefaultOptions()

	// Which collections keep their before/after VALUES.
	//
	// Every audited event records `changed_fields` either way -- the NAMES of
	// the fields that moved -- so leaving this empty still gives you who
	// changed what, and when. What it leaves out is the values themselves.
	//
	// That is the default because a snapshot copies every field a collection
	// does not mark `hidden`, and "not hidden" is not the same question as
	// "safe to keep a permanent second copy of": applications routinely leave a
	// credential readable so the identity owning it can fetch it back, and rely
	// on row-level API rules to decide who sees which row. The audit collection
	// has no row scoping, so a value protected by scoping is not protected once
	// it is in here -- and it outlives rotation, since the credential you
	// replaced because it leaked stays in before_changes.
	//
	// So list the collections whose diffs a human actually reads, and leave out
	// anything holding a secret. An allowlist, because a list of fields to
	// redact stops covering a sensitive field the moment someone adds one.
	options.SnapshotCollections = []string{"products", "orders"}

	if err := pbaudit.Setup(app, options); err != nil {
		log.Fatalf("Failed to setup audit logging: %v", err)
	}

	// Other options worth knowing about:
	//
	//   options.CollectionName = "my_audit_logs"
	//   options.LogAuthEvents = false
	//   options.LogToConsole = false
	//
	//   // Skip a collection entirely -- no event at all, not even field names.
	//   options.EventFilter = func(collectionName, eventType string) bool {
	//       return collectionName != "signing_keys"
	//   }
	//
	//   // Audit logs grow forever unless you say otherwise.
	//   options.Retention = &pbaudit.RetentionPolicy{
	//       MaxAge:   90 * 24 * time.Hour,
	//       Interval: "0 2 * * *",
	//   }

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
