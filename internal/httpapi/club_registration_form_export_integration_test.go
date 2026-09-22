//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/database"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestClubRegistrationConfiguredProfileExport(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	userData := objectData(t, f.call("POST", "/v2/members", map[string]string{"name": "Club education fixture"}, f.token, 201))
	var user database.Object
	if err := json.Unmarshal(userData["user"], &user); err != nil {
		t.Fatal(err)
	}
	club := objectData(t, f.call("POST", "/v2/clubs", map[string]string{"name": "Club education export"}, f.token, 200))
	var formID int32
	var provinceID, cityID int32
	defer func() {
		for _, target := range []struct {
			table string
			id    int32
		}{{"custom_forms", formID}, {"clubs", club.ID("id")}, {"public_users", user.ID("id")}, {"cities", cityID}, {"provinces", provinceID}} {
			if _, err := f.pool.Exec(ctx, "DELETE FROM "+target.table+" WHERE id=$1", target.id); err != nil {
				t.Error(err)
			}
		}
	}()
	province, city := "Fixture province", "Fixture city"
	provinceID = objectData(t, f.call("POST", "/v2/provinces", map[string]string{"name": province}, f.token, 200)).ID("id")
	cityID = objectData(t, f.call("POST", "/v2/cities", map[string]interface{}{"name": city, "province_id": provinceID}, f.token, 200)).ID("id")
	if _, err := f.pool.Exec(ctx, `UPDATE profiles SET gender='F',birth_date='2003-04-05',whatsapp='081234567890',origin_province_id=$2,origin_city_id=$3,province_id=$2,major='Stale major',education_history='[{"degree":"high_school","institution":"Previous school"},{"degree":"bachelor","institution":"Current campus","faculty":"Engineering","major":"Informatics","intake_year":2024}]' WHERE user_id=$1`, user.ID("id"), provinceID, cityID); err != nil {
		t.Fatal(err)
	}
	schema := `{"fields":[{"section_name":"profile_data","fields":[{"key":"name","label":"Nama Lengkap"},{"key":"gender","label":"Jenis Kelamin"},{"key":"birth_date","label":"Tanggal Lahir"},{"key":"origin_province_id","label":"Provinsi Asal"},{"key":"origin_city_id","label":"Kota Asal"},{"key":"province_id","label":"Provinsi Domisili"},{"key":"whatsapp","label":"WhatsApp"},{"key":"current_education","label":"Pendidikan Sekarang"}]},{"section_name":"Registrasi","fields":[{"key":"attendance","label":"Kesediaan hadir","options":[{"value":"06e119d1-e88e-4e57-8211-6b10a480d13d","label":"Bersedia"}]},{"key":"missing","label":"Belum diisi"}]}]}`
	if err := f.pool.QueryRow(ctx, "INSERT INTO custom_forms(form_name,feature_type,feature_id,form_schema,is_active,created_at,updated_at) VALUES('Education fixture','club_registration',$1,$2,true,now(),now()) RETURNING id", club.ID("id"), schema).Scan(&formID); err != nil {
		t.Fatal(err)
	}
	clubPath := fmt.Sprintf("/v2/clubs/%d", club.ID("id"))
	reg := objectData(t, f.call("POST", clubPath+"/registrations", map[string]interface{}{"member_id": user.ID("id"), "additional_data": map[string]interface{}{"attendance": "06e119d1-e88e-4e57-8211-6b10a480d13d", "legacy_answer": false}}, f.token, 200))
	detail := objectData(t, f.call("GET", fmt.Sprintf("/v2/club-registrations/%d", reg.ID("id")), nil, f.token, 200))
	profile := nestedObject(nestedObject(detail, "member"), "profile")
	var education []database.Object
	if err := json.Unmarshal(profile["education_history"], &education); err != nil || len(education) != 2 || education[1].String("institution") != "Current campus" {
		t.Fatalf("drawer profile missing current education: %s (%v)", profile["education_history"], err)
	}
	request := httptest.NewRequest("GET", clubPath+"/registrations/export", nil)
	request.Header.Set("Authorization", "Bearer "+f.token)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("export status %d: %s", response.Code, response.Body)
	}
	book, err := excelize.OpenReader(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows("Registrations")
	if err != nil || len(rows) != 2 {
		t.Fatalf("export rows: %v (%v)", rows, err)
	}
	wantHeaders := []string{"No", "Nama Lengkap", "Jenis Kelamin", "Tanggal Lahir", "Provinsi Asal", "Kota Asal", "Provinsi Domisili", "WhatsApp", "Pendidikan Sekarang - Kampus/Sekolah", "Pendidikan Sekarang - Jenjang", "Pendidikan Sekarang - Fakultas", "Pendidikan Sekarang - Jurusan", "Pendidikan Sekarang - Tahun Masuk", "Email", "Status", "Tanggal Pendaftaran", "Kesediaan hadir", "Belum diisi", "Legacy answer"}
	if !reflect.DeepEqual(rows[0], wantHeaders) {
		t.Fatalf("columns: %v", rows[0])
	}
	wantProfile := []string{"1", "Club education fixture", "Perempuan", "2003-04-05", province, city, province, "081234567890", "Current campus", "S1", "Engineering", "Informatics", "2024"}
	if !reflect.DeepEqual(rows[1][:len(wantProfile)], wantProfile) || rows[1][16] != "Bersedia" || rows[1][17] != "" || rows[1][18] != "Tidak" {
		t.Fatalf("export values: %v", rows[1])
	}
}
