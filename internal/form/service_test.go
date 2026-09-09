package form

import (
	"encoding/json"
	"kaderisasi/admin/internal/database"
	"testing"
)

func obj(raw string) database.Object {
	var result database.Object
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}
func TestAttachmentAndOpenFormChanges(t *testing.T) {
	current := obj(`{"feature_type":"club_registration","feature_id":12,"is_active":true,"form_schema":{"fields":[{"section_name":"profile","fields":[{"key":"name","label":"Name","required":true,"type":"text"}]}]}}`)
	if ClubID(current) != 12 || UpdatedClubID(current, obj(`{}`)) != 12 || UpdatedClubID(current, obj(`{"feature_id":24}`)) != 24 || UpdatedClubID(current, obj(`{"feature_type":"independent_form","feature_id":null}`)) != 0 {
		t.Fatal("partial attachment update")
	}
	if ClubID(obj(`{"feature_type":"club_registration","feature_id":null}`)) != 0 || ClubID(obj(`{"feature_type":"activity_registration","feature_id":12}`)) != 0 {
		t.Fatal("non-club attachment")
	}
	clone := database.Object{}
	clone["form_schema"] = current["form_schema"]
	if ChangesOpen(current, clone) {
		t.Fatal("equal schema protected")
	}
	for _, raw := range []string{`{"form_schema":{"fields":[]}}`, `{"is_active":false}`, `{"feature_id":null}`} {
		if !ChangesOpen(current, obj(raw)) {
			t.Error("unprotected", raw)
		}
	}
}
