package export

import (
	"encoding/json"
	"kaderisasi/admin/internal/member"
	"testing"
)

func TestRegistrationHistoryExport(t *testing.T) {
	raw, _ := json.Marshal(`[null,{"degree":"bachelor","institution":"ITB","intake_year":"2017"}]`)
	userID := int32(1)
	for _, registration := range []Registration{
		{UserID: &userID, Profile: &member.ProfileResponse{
			EducationHistory: raw,
			WorkHistory:      json.RawMessage(`[null,{"job_title":"Engineer","company":"Company","start_year":"2021"}]`),
		}},
		{Guest: RegistrationGuest{
			EducationHistory: raw,
			WorkHistory:      json.RawMessage(`[null,{"job_title":"Engineer","company":"Company","start_year":"2021"}]`),
		}},
	} {
		row, err := RegistrationRow(1, registration, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		if row[20] != "ITB" || row[23] != float64(2017) || row[24] != "bachelor - ITB - 2017" || row[25] != "Engineer - Company - 2021" {
			t.Fatalf("history columns: %#v", row[20:26])
		}
	}
	row, err := RegistrationRow(1, Registration{Guest: RegistrationGuest{
		EducationHistory: json.RawMessage(`[{"institution":"Old"}]`),
		CurrentEducation: json.RawMessage(`{"institution":"Selected"}`),
	}}, nil, "")
	if err != nil || row[20] != "Selected" {
		t.Fatalf("guest selection: %v %v", row, err)
	}
}
