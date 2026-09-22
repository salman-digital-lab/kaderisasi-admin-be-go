package export

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/member"
	"reflect"
	"testing"
)

func TestClubConfiguredProfileExport(t *testing.T) {
	schema := []byte(`{"fields":[{"section_name":"profile_data","fields":[{"key":"name","label":"Nama Lengkap"},{"key":"gender","label":"Jenis Kelamin"},{"key":"origin_province_id","label":"Provinsi Asal"},{"key":"origin_city_id","label":"Kota Asal"},{"key":"current_education","label":"Pendidikan Sekarang"}]}]}`)
	fields := ClubProfileFields(schema)
	headers := ClubProfileHeaders(fields)
	wantHeaders := []string{"Nama Lengkap", "Jenis Kelamin", "Provinsi Asal", "Kota Asal", "Pendidikan Sekarang - Kampus/Sekolah", "Pendidikan Sekarang - Jenjang", "Pendidikan Sekarang - Fakultas", "Pendidikan Sekarang - Jurusan", "Pendidikan Sekarang - Tahun Masuk", "Email"}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("headers: %v", headers)
	}
	gender, email, province, city, oldMajor := "F", "fixture@example.test", "Jawa Barat", "Bandung", "Old major"
	history := `[{"degree":"high_school","institution":"Old school"},null,{"degree":"bachelor","institution":"Current campus","faculty":"Engineering","major":"Informatics","intake_year":"2024"}]`
	for _, serialized := range []bool{false, true} {
		raw := json.RawMessage(history)
		if serialized {
			raw, _ = json.Marshal(history)
		}
		profile := &member.ProfileResponse{Profile: dbgen.Profile{Name: "Applicant", Gender: &gender, Major: &oldMajor}, EducationHistory: raw}
		row := ClubProfileRow(fields, profile, &email, RegistrationLocations{OriginProvince: &province, OriginCity: &city})
		want := []interface{}{"Applicant", "Perempuan", "Jawa Barat", "Bandung", "Current campus", "S1", "Engineering", "Informatics", float64(2024), email}
		if !reflect.DeepEqual(row, want) {
			t.Fatalf("serialized=%v row: %#v", serialized, row)
		}
	}
	missing := ClubProfileRow(fields, nil, &email, RegistrationLocations{})
	if len(missing) != len(headers) || missing[len(missing)-1] != email || missing[5] != "" {
		t.Fatalf("missing profile misaligned: %v", missing)
	}
}

func TestClubProfileFallbackAndEmptyForm(t *testing.T) {
	fields := ClubProfileFields(nil)
	want := []string{"Nama Lengkap", "Email", "Whatsapp", "Nomor Identitas", "Provinsi", "Universitas", "Jurusan", "Tahun Masuk", "Jenjang"}
	if !reflect.DeepEqual(ClubProfileHeaders(fields), want) {
		t.Fatalf("legacy headers: %v", ClubProfileHeaders(fields))
	}
	empty := []byte(`{"fields":[{"section_name":"profile_data","fields":[]}]}`)
	if got := ClubProfileHeaders(ClubProfileFields(empty)); !reflect.DeepEqual(got, []string{"Email"}) {
		t.Fatalf("empty profile section unexpectedly exports unrequested data: %v", got)
	}
}

func TestClubChoiceLabelsAndProfileExclusion(t *testing.T) {
	schema := []byte(`{"fields":[{"section_name":"profile_data","fields":[{"key":"current_education","label":"Pendidikan Sekarang"}]},{"section_name":"Registrasi","fields":[{"key":"attendance","label":"Kesediaan hadir","options":[{"value":"06e119d1-e88e-4e57-8211-6b10a480d13d","label":"Bersedia"},{"value":0,"label":"Belum pernah"}]}]}]}`)
	questions := ClubQuestions(schema, [][]byte{[]byte(`{"current_education":{},"attendance":[],"legacy_answer":"Old"}`)})
	if len(questions) != 2 || questions[0].Key != "attendance" || questions[1].Key != "legacy_answer" {
		t.Fatalf("questions: %#v", questions)
	}
	for _, test := range []struct{ raw, want string }{
		{`"06e119d1-e88e-4e57-8211-6b10a480d13d"`, "Bersedia"},
		{`["06e119d1-e88e-4e57-8211-6b10a480d13d","0","Old choice"]`, "Bersedia, Belum pernah, Old choice"},
		{`null`, ""}, {`false`, "Tidak"},
	} {
		if got := questions[0].Answer(json.RawMessage(test.raw)); got != test.want {
			t.Errorf("%s = %#v", test.raw, got)
		}
	}
}
