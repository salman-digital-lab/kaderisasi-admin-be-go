package member

import (
	"encoding/json"
	"testing"

	"kaderisasi/admin/internal/dbgen"
)

func TestProfileDiffReportsOnlyChangedFields(t *testing.T) {
	gender, phone := "M", "0811"
	before := dbgen.Profile{Name: "Ahmad", Gender: &gender, Badges: []byte(`["a","b"]`), ExtraData: []byte(`{"preferred_name":"Mad","alumni_regional_assignment":["Bandung"]}`)}
	after := before
	after.Name = "Ahmad Fauzi"
	after.Whatsapp = &phone
	// Same JSON, different spacing and key order: not a change.
	after.Badges = []byte(`[ "a", "b" ]`)
	after.ExtraData = []byte(`{"alumni_regional_assignment":["Jakarta"],"preferred_name":"Mad","address_detail":"Jl. Ganesha"}`)

	changes := diffFields(profileFields(before), profileFields(after))
	want := map[string][2]string{
		"name":                                  {`"Ahmad"`, `"Ahmad Fauzi"`},
		"whatsapp":                              {`null`, `"0811"`},
		"extra_data.alumni_regional_assignment": {`["Bandung"]`, `["Jakarta"]`},
		"extra_data.address_detail":             {`null`, `"Jl. Ganesha"`},
	}
	if len(changes) != len(want) {
		t.Fatalf("changes = %v", changes)
	}
	for field, values := range want {
		got, ok := changes[field]
		if !ok || string(got.Before) != values[0] || string(got.After) != values[1] || got.Changed {
			t.Fatalf("%s = %s -> %s", field, got.Before, got.After)
		}
	}
	if len(diffFields(profileFields(before), profileFields(before))) != 0 {
		t.Fatal("an unchanged profile reports changes")
	}
}

func TestProfileDiffKeepsLegacyExtraDataWhole(t *testing.T) {
	changes := diffFields(profileFields(dbgen.Profile{ExtraData: []byte(`"legacy"`)}), profileFields(dbgen.Profile{ExtraData: []byte(`{"preferred_name":"Mad"}`)}))
	if string(changes["extra_data"].Before) != `"legacy"` || string(changes["extra_data.preferred_name"].After) != `"Mad"` {
		t.Fatalf("changes = %v", changes)
	}
}

func TestDeletedAndCreatedSnapshotsSkipEmptyFields(t *testing.T) {
	name := "Siti"
	deleted := diffFields(profileFields(dbgen.Profile{Name: name, Badges: []byte(`[]`), ExtraData: []byte(`{}`)}), nil)
	if len(deleted) != 2 || string(deleted["name"].Before) != `"Siti"` || string(deleted["name"].After) != "null" || string(deleted["badges"].Before) != "[]" {
		t.Fatalf("deleted = %v", deleted)
	}
	user := int32(7)
	created := diffFields(nil, createdFields(dbgen.CreateMemberProfileRow{ID: 3, UserID: &user, Name: name}))
	if len(created) != 1 || string(created["name"].After) != `"Siti"` {
		t.Fatalf("created = %v", created)
	}
}

// A password never reaches the trail: only the fact that it changed.
func TestPasswordChangeCarriesNoValue(t *testing.T) {
	raw, err := json.Marshal(AuditChanges{"password": passwordChanged})
	if err != nil || string(raw) != `{"password":{"changed":true}}` {
		t.Fatalf("%s %v", raw, err)
	}
	old, current := "a@example.test", "b@example.test"
	changes := AuditChanges{}
	emailChange(changes, &old, &current)
	emailChange(changes, &old, &old)
	if len(changes) != 1 || string(changes["email"].After) != `"b@example.test"` {
		t.Fatalf("changes = %v", changes)
	}
}
