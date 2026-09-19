package audit

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/pocketbase/pocketbase/core"
)

// envelopeKeys are the entries PublicExport adds that are NOT record fields.
//
// PocketBase appends the collection reference and, when relations have been
// expanded, the expanded records themselves. None of them is a field of the
// record: the first two are constant for the collection and can never move,
// and `expand` is a copy of OTHER records that happened to be loaded alongside
// this one -- so naming it would report a change that did not happen to this
// record, and would do it using the contents of a different one.
var envelopeKeys = map[string]bool{
	core.FieldNameCollectionId:   true,
	core.FieldNameCollectionName: true,
	core.FieldNameExpand:         true,
}

// changedFields names the fields whose value differs between two record
// snapshots, sorted so the stored array is stable between events.
//
// WHY THIS EXISTS, AND WHY IT IS THE DEFAULT. A before/after snapshot copies
// every non-hidden field of a record into the audit log, which means the audit
// collection accumulates a plaintext copy of anything the application stores
// and does not mark hidden. That is fine for a name or a description and very
// much not fine for a credential: an application can have perfectly good
// reasons NOT to hide a field -- the owner of the record has to be able to read
// it back -- while still not wanting a permanent second copy of every version
// of it in a collection with no row scoping.
//
// The field NAMES carry most of the forensic value at none of that cost.
// "user X updated api_keys/abc123, fields: [secret, rotated_at]" answers who,
// what and when, and does not hand the reader the secret. So names are recorded
// always, and values only for the collections named in
// Options.SnapshotCollections.
//
// An ALLOWLIST rather than a list of fields to redact, deliberately: a deny
// list silently stops covering a sensitive field the moment someone adds one,
// which is the failure mode this is meant to close rather than reproduce.
//
// A nil snapshot is read as an empty record, so one function covers all three
// shapes: a create names the fields that were populated, a delete names the
// fields that were lost, and an update names the true difference.
func changedFields(before, after map[string]any) []string {
	names := make(map[string]struct{}, len(before)+len(after))
	for name := range before {
		names[name] = struct{}{}
	}
	for name := range after {
		names[name] = struct{}{}
	}

	changed := make([]string, 0, len(names))
	for name := range names {
		if envelopeKeys[name] {
			continue
		}
		if !sameValue(before[name], after[name]) {
			changed = append(changed, name)
		}
	}

	sort.Strings(changed)
	return changed
}

// sameValue reports whether two field values are equivalent for audit purposes.
//
// Compared as JSON rather than with reflect.DeepEqual, because these values
// arrive from Record.PublicExport() as interface{} holding PocketBase's own
// types -- types.DateTime, types.JSONRaw, relation slices -- where two values
// that serialise identically can differ structurally. JSON is also exactly the
// form the snapshot would be stored in, so the comparison agrees with what a
// reader of before_changes/after_changes would see. Go's encoder sorts map
// keys, so the encoding is deterministic.
//
// ABSENT AND ZERO ARE THE SAME THING HERE. Without that, every create would
// name every field: PublicExport emits a key for each field including the
// empty ones, the other side is an absent map, and "" differs from nothing at
// all. Treating both as unset makes a create name the fields that were
// actually populated -- which is the question someone reading a create event is
// asking -- and leaves genuine transitions intact, since "x" -> "" and
// true -> false each have a non-empty side.
func sameValue(a, b any) bool {
	aJSON, aErr := json.Marshal(a)
	bJSON, bErr := json.Marshal(b)

	// A value that will not serialise is reported as CHANGED rather than
	// assumed equal. An audit trail that quietly omits a field because it could
	// not be compared is worse than one that names a field which did not move.
	if aErr != nil || bErr != nil {
		return false
	}

	if bytes.Equal(aJSON, bJSON) {
		return true
	}
	return isEmptyJSON(aJSON) && isEmptyJSON(bJSON)
}

// isEmptyJSON reports whether an encoded value is the zero value for its type.
//
// A short explicit list rather than reflection on the original value: these are
// the only encodings PocketBase field types produce for "unset", and a list you
// can read in one glance is easier to trust than a rule you have to derive.
func isEmptyJSON(raw []byte) bool {
	switch string(raw) {
	case "null", `""`, "0", "false", "[]", "{}":
		return true
	default:
		return false
	}
}
