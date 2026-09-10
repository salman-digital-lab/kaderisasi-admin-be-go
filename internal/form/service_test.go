package form

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"testing"
)

func TestAttachmentAndOpenFormChanges(t *testing.T) {
	kind, id, active := "club_registration", int32(12), true
	current := dbgen.CustomForm{FeatureType: &kind, FeatureID: &id, IsActive: &active, FormSchema: []byte(`{"fields":[{"section_name":"profile","fields":[{"key":"name","label":"Name","required":true,"type":"text"}]}]}`)}
	if *currentClub(current) != "12" {
		t.Fatal("club attachment")
	}
	for _, test := range []struct {
		raw       string
		club      *string
		protected bool
	}{
		{`{}`, new("12"), false},
		{`{"featureId":24}`, new("24"), true},
		{`{"featureType":"independent_form","featureId":null}`, nil, true},
		{`{"featureId":null}`, nil, true},
		{`{"isActive":false}`, new("12"), true},
		{`{"formSchema":{"fields":[]}}`, new("12"), true},
		{`{"formName":"Renamed"}`, new("12"), false},
		{`{"formSchema":{"fields":[{"fields":[{"type":"text","required":true,"label":"Name","key":"name"}],"section_name":"profile"}]}}`, new("12"), false},
	} {
		t.Run(test.raw, func(t *testing.T) {
			var input Input
			if err := json.Unmarshal([]byte(test.raw), &input); err != nil {
				t.Fatal(err)
			}
			next, err := merged(current, input)
			if err != nil {
				t.Fatal(err)
			}
			got := attachedClub(next.FeatureType, next.FeatureID)
			if (got == nil) != (test.club == nil) || (got != nil && *got != *test.club) {
				t.Fatal("attachment", got, test.club)
			}
			if changesOpen(current, next) != test.protected {
				t.Fatal("protected changes")
			}
		})
	}
	zero := int32(0)
	current.FeatureID = &zero
	if got := currentClub(current); got == nil || *got != "0" {
		t.Fatal("zero is not null in attachment service")
	}
	current.FeatureID = nil
	if currentClub(current) != nil {
		t.Fatal("null attachment")
	}
	current.FeatureID = &id
	current.FeatureType = new("activity_registration")
	if currentClub(current) != nil {
		t.Fatal("non-club attachment")
	}
	current.FormSchema = nil
	next, err := merged(current, Input{FeatureID: domain.Optional[json.Number]{}})
	if err != nil || changesOpen(current, next) {
		t.Fatal("omitted nullable schema changed", err)
	}
}
func TestSafeFormIdentifiers(t *testing.T) {
	for _, raw := range []string{"1", "01", "1e0", "0x1", "0b1", "0o1", " 1 ", "1.0"} {
		if id, err := safeIdentifier(raw); err != nil || id != "1" {
			t.Fatal(raw, id, err)
		}
	}
	for _, raw := range []string{"", "0", "-1", "1.5", "bad", "NaN", "Infinity", "9007199254740992", "%31"} {
		if _, err := safeIdentifier(raw); err == nil {
			t.Fatal("accepted", raw)
		}
	}
	if id, err := safeIdentifier("2147483648"); err != nil || id != "2147483648" {
		t.Fatal("narrowed wide safe integer", id, err)
	}
}
