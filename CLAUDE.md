# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

pb-audit is a Go library that provides audit logging for PocketBase applications. It tracks record operations (create/update/delete), API requests, and authentication events using a dual-tracking system (request events before commit + success events after commit).

Every event records `changed_fields` — the NAMES of the fields that moved. Full before/after VALUES are opt-in per collection via `Options.SnapshotCollections`, and the default is to store none. See "Why values are opt-in" below; it is the single most important thing to understand before changing this library.

Module: `github.com/skeeeon/pb-audit`
Requires: Go 1.24+, PocketBase v0.31+

## Build & Test Commands

```bash
go build ./...          # Build all packages
go vet ./...            # Lint
go test ./...           # Run all tests
go test -run TestName ./internal/audit/  # Run a single test
```

CI (`.github/workflows/ci.yml`) runs gofmt, vet and tests on every push and PR.
Releases are tag-driven: pushing a `v*` tag runs vet/test/build and then
goreleaser (`.github/workflows/release.yml`). There is no version constant in
the source — a library has no ldflags equivalent, so a hardcoded string only
drifts from the tag. Consumers read the version from `debug.ReadBuildInfo()`.

## Architecture

The library exposes a single public entry point: `pbaudit.Setup(app, options)` in `audit.go` at the package root. This converts public `Options` to internal options and binds to PocketBase's `OnBootstrap` event.

All implementation lives in `internal/audit/`:

- **audit.go** — `Initialize()` orchestrates setup: checks if the audit collection exists, creates it if needed, then registers hooks
- **collections.go** — `ensureAuditCollection()` creates the `audit_logs` PocketBase collection with schema, indexes, and admin-only API rules (non-destructive: skips if collection already exists)
- **hooks.go** — `registerHooks()` registers three hook categories on the PocketBase app: request hooks (pre-commit with full request context), success hooks (post-commit confirmation), and auth hooks (login tracking). Helper `extractRequestInfo()` pulls IP/method/URL/user from request events
- **logger.go** — `logger` type with `logEvent()` that creates audit log records. Contains `shouldLogEvent()` for filtering, `isValidUser()` for user relation validation, `setSnapshot()` for the opt-in value copy, and `extractClientIP()` for proxy-aware IP extraction (CF > X-Forwarded-For > X-Real-IP > Fly-Client-IP)
- **diff.go** — `changedFields()` compares two `PublicExport()` maps and returns the sorted names of the fields that differ. Pure logic with no PocketBase app required, which is why it carries the library's tests (`diff_test.go`)
- **constants.go** — Event type constants and `AuditLogFields` struct mapping field names

## Key Design Decisions

- **Why values are opt-in**: a snapshot is `Record.PublicExport()` — every field the collection does not mark `hidden`. "Not hidden" is not the same question as "safe to keep a permanent second copy of": applications routinely leave a credential readable so the identity owning it can fetch it back, and rely on row-level API rules to decide who sees which row. The audit collection has no row scoping, so a value protected by scoping is not protected once it lands here, and it outlives rotation — the credential replaced because it leaked stays in `before_changes`. Field names carry the forensic value without the copy. `SnapshotCollections` is an **allowlist** rather than a set of fields to redact, because a deny list silently stops covering a sensitive field the moment someone adds one.
- **Absent and zero compare equal** in the diff: without it every create names every field, since `PublicExport` emits a key per field while the other side is an absent map. Genuine transitions are unaffected — `"x" -> ""` and `true -> false` each have a non-empty side. `TestChangedFields` pins both directions.
- **The diff skips `collectionId`, `collectionName` and `expand`**: they are export envelope, not fields of the record. `expand` matters most — it is a copy of OTHER records, so naming it would report a change that did not happen here using data from somewhere else.
- **Values are compared as JSON**, not with `reflect.DeepEqual`: they arrive as PocketBase's own types (`types.DateTime`, `types.JSONRaw`), where equal values can differ structurally. A value that will not serialise is reported as CHANGED — an audit trail that quietly omits a field is worse than one that over-names.
- **`Record.Original()` is the before state on update success**: PocketBase does not refresh a record's original data on save, so the pre-save values survive into the after-success hook. This is what lets a programmatic `app.Save()` produce a diff at all, since no request hook fired for it. It degrades in the safe direction — a record built in memory and saved twice without reloading has an empty `Original()`, so the diff over-reports rather than hiding a change.
- **Dual-tracking**: Request events capture intent (with before-state); success events confirm commit (with after-state). Both are needed for a complete audit trail.
- **Non-destructive setup**: Collection is only created on first run; subsequent starts skip schema changes to preserve user customizations.
- **Audit failures never block**: Logging errors are printed to console but don't halt the application. This is why audit hooks live on the After*Success side and must never be used as an enforcement point — enforcement has the opposite requirement.
- **User field is optional**: Admin/superuser operations result in null user (admins aren't in the users collection).
- **Record ID empty on create_request**: The record hasn't been saved yet, so no ID exists.

## Contributing Philosophy

From README: follow "grug brained developer" philosophy — simple explicit code, clear documentation, one file one purpose.
