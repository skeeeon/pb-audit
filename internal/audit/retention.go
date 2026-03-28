package audit

import (
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase"
)

const (
	// retentionJobID is the cron job identifier for the retention cleanup task.
	retentionJobID = "pb_audit_retention"

	// retentionBatchSize is the number of records to delete per batch
	// to avoid loading too many records into memory at once.
	retentionBatchSize = 100
)

// registerRetention registers a cron job that periodically cleans up old audit logs.
//
// The cron job runs on the schedule defined by options.Retention.Interval and enforces
// both MaxAge and MaxRecords constraints (whichever are set).
func registerRetention(app *pocketbase.PocketBase, options Options) error {
	retention := options.Retention

	// Nothing to do if neither constraint is set
	if retention.MaxAge <= 0 && retention.MaxRecords <= 0 {
		return nil
	}

	app.Cron().MustAdd(retentionJobID, retention.Interval, func() {
		runRetention(app, options)
	})

	if options.LogToConsole {
		fmt.Printf("✅ SUCCESS Retention policy cron job registered (schedule: %s)\n", retention.Interval)
	}

	return nil
}

// runRetention executes the retention cleanup logic.
//
// It enforces two independent constraints:
// 1. MaxAge: deletes records with timestamp older than Now() - MaxAge
// 2. MaxRecords: if total count exceeds MaxRecords, deletes the oldest excess records
//
// Errors are logged but never propagated — retention failures must not affect the application.
func runRetention(app *pocketbase.PocketBase, options Options) {
	retention := options.Retention

	if options.LogToConsole {
		fmt.Println("🧹 AUDIT  Running retention cleanup...")
	}

	// Age-based cleanup
	if retention.MaxAge > 0 {
		deleted := deleteByAge(app, options)
		if options.LogToConsole && deleted > 0 {
			fmt.Printf("🧹 AUDIT  Deleted %d records exceeding max age (%v)\n", deleted, retention.MaxAge)
		}
	}

	// Count-based cleanup
	if retention.MaxRecords > 0 {
		deleted := deleteByCount(app, options)
		if options.LogToConsole && deleted > 0 {
			fmt.Printf("🧹 AUDIT  Deleted %d records exceeding max count (%d)\n", deleted, retention.MaxRecords)
		}
	}

	if options.LogToConsole {
		fmt.Println("🧹 AUDIT  Retention cleanup complete")
	}
}

// deleteByAge deletes audit records older than MaxAge in batches.
// Returns the total number of records deleted.
func deleteByAge(app *pocketbase.PocketBase, options Options) int {
	cutoff := time.Now().Add(-options.Retention.MaxAge).UTC().Format("2006-01-02 15:04:05.000Z")
	filter := fmt.Sprintf("%s < '%s'", AuditLogFields.Timestamp, cutoff)

	totalDeleted := 0
	for {
		records, err := app.FindRecordsByFilter(
			options.CollectionName,
			filter,
			AuditLogFields.Timestamp,
			retentionBatchSize,
			0,
		)
		if err != nil {
			if options.LogToConsole {
				fmt.Printf("⚠️  WARNING Retention age query failed: %v\n", err)
			}
			break
		}

		if len(records) == 0 {
			break
		}

		for _, record := range records {
			if err := app.Delete(record); err != nil {
				if options.LogToConsole {
					fmt.Printf("⚠️  WARNING Retention failed to delete record %s: %v\n", record.Id, err)
				}
			} else {
				totalDeleted++
			}
		}
	}

	return totalDeleted
}

// deleteByCount deletes the oldest audit records that exceed MaxRecords.
// Returns the total number of records deleted.
func deleteByCount(app *pocketbase.PocketBase, options Options) int {
	total, err := app.CountRecords(options.CollectionName)
	if err != nil {
		if options.LogToConsole {
			fmt.Printf("⚠️  WARNING Retention count query failed: %v\n", err)
		}
		return 0
	}

	excess := int(total) - options.Retention.MaxRecords
	if excess <= 0 {
		return 0
	}

	totalDeleted := 0
	remaining := excess

	for remaining > 0 {
		batchSize := retentionBatchSize
		if remaining < batchSize {
			batchSize = remaining
		}

		records, err := app.FindRecordsByFilter(
			options.CollectionName,
			"",
			AuditLogFields.Timestamp,
			batchSize,
			0,
		)
		if err != nil {
			if options.LogToConsole {
				fmt.Printf("⚠️  WARNING Retention count-based query failed: %v\n", err)
			}
			break
		}

		if len(records) == 0 {
			break
		}

		for _, record := range records {
			if err := app.Delete(record); err != nil {
				if options.LogToConsole {
					fmt.Printf("⚠️  WARNING Retention failed to delete record %s: %v\n", record.Id, err)
				}
			} else {
				totalDeleted++
			}
		}

		remaining -= len(records)
	}

	return totalDeleted
}
