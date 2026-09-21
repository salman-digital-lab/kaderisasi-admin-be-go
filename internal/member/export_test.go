package member

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"reflect"
	"testing"
)

func TestExportColumnsCoverProfileAndRejectAccountSecrets(t *testing.T) {
	profile := reflect.TypeFor[dbgen.Profile]()
	keys := map[string]bool{}
	for _, column := range exportColumns {
		keys[column.Key] = true
	}
	for i := 0; i < profile.NumField(); i++ {
		if !keys[profile.Field(i).Tag.Get("json")] {
			t.Fatalf("missing profile field %s", profile.Field(i).Name)
		}
	}
	for _, columns := range [][]string{nil, {"password"}, {"email", "email"}, {"publicUser"}, {"unknown"}} {
		if _, err := exportHeaders(columns, "csv"); err == nil {
			t.Fatalf("accepted invalid columns: %v", columns)
		}
	}
	if _, err := exportHeaders([]string{"email"}, "pdf"); err == nil {
		t.Fatal("accepted unsupported format")
	}
}

func TestExportValuesPreserveRawProfileData(t *testing.T) {
	phone := "00123456789"
	row := dbgen.ListProfilesFilteredRow{Profile: dbgen.Profile{
		Name: "Nama, dengan\nbaris baru", Whatsapp: &phone,
		Badges: []byte(`["LMD"]`), EducationHistory: []byte(`[{"institution":"ITB"}]`),
		ExtraData: []byte(`{"number":9007199254740993}`),
	}, PublicUser: []byte(`{"email":"member@example.test","password":"never export"}`)}
	values, err := exportValues(row, []string{"whatsapp", "email", "education_history", "extra_data", "gender"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{phone, "member@example.test", `[{"institution":"ITB"}]`, `{"number":9007199254740993}`, ""}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("got %v want %v", values, want)
	}
	row.PublicUser = json.RawMessage(`null`)
	values, err = exportValues(row, []string{"email"})
	if err != nil || values[0] != "" {
		t.Fatalf("missing account: %v %v", values, err)
	}
}
