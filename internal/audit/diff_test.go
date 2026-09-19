package audit

import (
	"reflect"
	"testing"

	"github.com/pocketbase/pocketbase/tools/types"
)

func TestChangedFields(t *testing.T) {
	cases := []struct {
		what   string
		before map[string]any
		after  map[string]any
		want   []string
	}{
		{
			what:   "an update names only the field that moved",
			before: map[string]any{"name": "old", "active": true, "notes": ""},
			after:  map[string]any{"name": "new", "active": true, "notes": ""},
			want:   []string{"name"},
		},
		{
			// The reason sameValue treats absent and zero alike. Without it a
			// create names every field the collection has, and the answer to
			// "what did they actually set" is buried.
			what:   "a create names the fields that were populated, not every field",
			before: nil,
			after:  map[string]any{"name": "thing", "description": "", "count": 0, "active": false, "tags": []any{}},
			want:   []string{"name"},
		},
		{
			what:   "a delete names the fields that were lost",
			before: map[string]any{"name": "thing", "description": ""},
			after:  nil,
			want:   []string{"name"},
		},
		{
			what:   "a save that moved nothing names nothing",
			before: map[string]any{"name": "same", "active": true},
			after:  map[string]any{"name": "same", "active": true},
			want:   []string{},
		},
		{
			// Both directions of a genuine transition through the zero value:
			// only one side is empty, so neither is swallowed.
			what:   "clearing a field is a change",
			before: map[string]any{"name": "was set"},
			after:  map[string]any{"name": ""},
			want:   []string{"name"},
		},
		{
			what:   "turning a flag off is a change",
			before: map[string]any{"active": true},
			after:  map[string]any{"active": false},
			want:   []string{"active"},
		},
		{
			what:   "a field that was already unset and stays unset is not a change",
			before: map[string]any{"notes": ""},
			after:  map[string]any{},
			want:   []string{},
		},
		{
			what:   "a field appearing on only one side is still compared",
			before: map[string]any{},
			after:  map[string]any{"role": "owner"},
			want:   []string{"role"},
		},
		{
			what:   "names come back sorted, so the stored array is stable",
			before: map[string]any{"zeta": "a", "alpha": "a", "mid": "a"},
			after:  map[string]any{"zeta": "b", "alpha": "b", "mid": "b"},
			want:   []string{"alpha", "mid", "zeta"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.what, func(t *testing.T) {
			got := changedFields(tc.before, tc.after)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("changedFields() = %v, want %v", got, tc.want)
			}
		})
	}
}

// PublicExport hands back PocketBase's own types, not plain Go values, which is
// why the comparison serialises rather than using reflect.DeepEqual: two of
// these can carry the same value in different internal shapes.
func TestChangedFieldsHandlesPocketBaseTypes(t *testing.T) {
	when, err := types.ParseDateTime("2026-01-02 15:04:05.000Z")
	if err != nil {
		t.Fatal(err)
	}
	same, err := types.ParseDateTime("2026-01-02 15:04:05.000Z")
	if err != nil {
		t.Fatal(err)
	}
	later, err := types.ParseDateTime("2026-03-04 15:04:05.000Z")
	if err != nil {
		t.Fatal(err)
	}

	got := changedFields(
		map[string]any{
			"expires_at": when,
			"metadata":   types.JSONRaw(`{"a":1,"b":2}`),
			"untouched":  types.JSONRaw(`{"k":"v"}`),
		},
		map[string]any{
			"expires_at": later,
			"metadata":   types.JSONRaw(`{"a":1,"b":3}`),
			"untouched":  types.JSONRaw(`{"k":"v"}`),
		},
	)

	want := []string{"expires_at", "metadata"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("changedFields() = %v, want %v", got, want)
	}

	if !sameValue(when, same) {
		t.Error("two equal DateTimes should compare equal")
	}
}

// The whole point of the change: a credential that is deliberately NOT hidden
// (because the identity owning it has to read it back) is named but never
// copied.
func TestChangedFieldsNamesASecretWithoutCarryingIt(t *testing.T) {
	const secret = "-----BEGIN USER NKEY SEED-----\nSUAKSECRET\n------END USER NKEY SEED------"

	got := changedFields(
		map[string]any{"creds_file": "", "username": "device-1"},
		map[string]any{"creds_file": secret, "username": "device-1"},
	)

	if !reflect.DeepEqual(got, []string{"creds_file"}) {
		t.Fatalf("changedFields() = %v, want [creds_file]", got)
	}
	for _, name := range got {
		if name == secret {
			t.Fatal("the diff carried the value rather than the field name")
		}
	}
}

// PublicExport appends the collection reference and any expanded relations.
// None of them is a field of the record being audited, and `expand` is a copy
// of OTHER records -- so reporting it would describe a change that did not
// happen here, using data from somewhere else.
func TestChangedFieldsIgnoresTheExportEnvelope(t *testing.T) {
	got := changedFields(
		map[string]any{
			"name":           "old",
			"collectionId":   "pbc_123",
			"collectionName": "things",
		},
		map[string]any{
			"name":           "new",
			"collectionId":   "pbc_123",
			"collectionName": "things",
			"expand":         map[string]any{"nats_user": map[string]any{"creds_file": "SECRET"}},
		},
	)

	if !reflect.DeepEqual(got, []string{"name"}) {
		t.Errorf("changedFields() = %v, want [name]", got)
	}
}
