//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/xuri/excelize/v2"
	"kaderisasi/admin/internal/database"
	"net/http/httptest"
	"testing"
)

func TestClubRegistrationRolesAndExport(t *testing.T) {
	t.Setenv("TZ", "UTC")
	f := newHTTPFixture(t)
	ctx := context.Background()
	users := []int32{}
	clubs := []int32{}
	defer func() {
		for _, target := range []struct {
			table string
			ids   []int32
		}{{"clubs", clubs}, {"public_users", users}} {
			if _, err := f.pool.Exec(ctx, "DELETE FROM "+target.table+" WHERE id=ANY($1::int[])", target.ids); err != nil {
				t.Error(err)
			}
		}
	}()
	data := objectData(t, f.call("POST", "/v2/members", map[string]string{"name": "Club participant"}, f.token, 201))
	var user database.Object
	json.Unmarshal(data["user"], &user)
	users = append(users, user.ID("id"))
	c := objectData(t, f.call("POST", "/v2/clubs", map[string]string{"name": "Club roles fixture"}, f.token, 200))
	clubs = append(clubs, c.ID("id"))
	clubPath := fmt.Sprintf("/v2/clubs/%d", c.ID("id"))
	payload := map[string]interface{}{"member_id": user.ID("id"), "additional_data": map[string]interface{}{"available": false, "count": 0, "old_question": []string{"A", "B"}}}
	reg := objectData(t, f.call("POST", clubPath+"/registrations", payload, f.token, 200))
	regPath := fmt.Sprintf("/v2/club-registrations/%d", reg.ID("id"))
	f.call("POST", clubPath+"/registrations", payload, f.token, 409)
	roleBody := map[string]interface{}{"club_registration_id": reg.ID("id"), "role_name": "Chairperson", "is_primary": true}
	f.call("POST", clubPath+"/member-roles", roleBody, f.token, 400)
	f.call("PUT", regPath, map[string]interface{}{"status": "APPROVED"}, f.token, 200)
	primary := objectData(t, f.call("POST", clubPath+"/member-roles", roleBody, f.token, 201))
	roleBody["role_name"] = "Secretary"
	secondary := objectData(t, f.call("POST", clubPath+"/member-roles", roleBody, f.token, 201))
	var isPrimary bool
	if err := f.pool.QueryRow(ctx, "SELECT is_primary FROM club_member_roles WHERE id=$1", primary.ID("id")).Scan(&isPrimary); err != nil || isPrimary {
		t.Fatal("primary role replacement", err)
	}
	f.call("PUT", fmt.Sprintf("/v2/club-registrations/member-roles/%d", primary.ID("id")), map[string]interface{}{"is_primary": true, "sort_order": -1, "start_date": "2026-01-01"}, f.token, 200)
	for _, path := range []string{clubPath + "/registrations?status=APPROVED&sort_order=asc&limit=1", clubPath + "/members?search=participant", regPath, clubPath + "/member-roles", clubPath + "/member-role-suggestions"} {
		f.call("GET", path, nil, f.token, 200)
	}
	f.call("PUT", "/v2/club-registrations/bulk-update", map[string]interface{}{"registrations": []interface{}{map[string]interface{}{"id": reg.ID("id"), "status": "REJECTED"}, map[string]interface{}{"id": reg.ID("id"), "status": "APPROVED"}}}, f.token, 400)
	f.call("PUT", "/v2/club-registrations/bulk-update", map[string]interface{}{"registrations": []interface{}{map[string]interface{}{"id": reg.ID("id"), "status": "REJECTED"}, map[string]interface{}{"id": 2147483647, "status": "APPROVED"}}}, f.token, 404)
	after := objectData(t, f.call("GET", regPath, nil, f.token, 200))
	var preserved map[string]interface{}
	if err := json.Unmarshal(after["additional_data"], &preserved); err != nil {
		t.Fatal(err)
	}
	if preserved["available"] != false || preserved["count"] != float64(0) {
		t.Fatalf("review changed submitted answers: %v", preserved)
	}
	if after.String("status") != "APPROVED" {
		t.Fatal("bulk missing ID changed other registrations")
	}
	f.call("PUT", "/v2/club-registrations/bulk-update", map[string]interface{}{"registrations": []interface{}{map[string]interface{}{"id": reg.ID("id"), "status": "REJECTED"}}}, f.token, 200)
	if _, err := f.pool.Exec(ctx, "UPDATE club_registrations SET created_at='2026-02-28T17:00:00Z' WHERE id=$1", reg.ID("id")); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", clubPath+"/registrations/export", nil)
	r.Header.Set("Authorization", "Bearer "+f.token)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("club export %d %s", w.Code, w.Body)
	}
	book, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows("Registrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[1][1] != "Club participant" || rows[1][10] != "REJECTED" {
		t.Fatalf("club export rows %v", rows)
	}
	cells := map[string]string{}
	for i, key := range rows[0] {
		if i < len(rows[1]) {
			cells[key] = rows[1][i]
		}
	}
	if cells["Available"] != "Tidak" || cells["Count"] != "0" || cells["Old question"] != "A, B" {
		t.Fatalf("export answer values %v", cells)
	}
	if cells["Tanggal Pendaftaran"] != "2026-03-01 00:00:00" {
		t.Fatalf("export ignored Jakarta timezone: %v", cells)
	}
	if _, err := f.pool.Exec(ctx, "UPDATE club_registrations SET created_at=NULL WHERE id=$1", reg.ID("id")); err != nil {
		t.Fatal(err)
	}
	legacy := httptest.NewRecorder()
	f.handler.ServeHTTP(legacy, r.Clone(ctx))
	if legacy.Code != 200 {
		t.Fatalf("legacy date export: %d %s", legacy.Code, legacy.Body)
	}
	legacyBook, err := excelize.OpenReader(bytes.NewReader(legacy.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer legacyBook.Close()
	date, err := legacyBook.GetCellValue("Registrations", "L2")
	if err != nil || date != "" {
		t.Fatalf("missing registration date: %q %v", date, err)
	}
	f.call("DELETE", fmt.Sprintf("/v2/club-registrations/member-roles/%d", secondary.ID("id")), nil, f.token, 200)
	f.call("DELETE", regPath, nil, f.token, 200)
	f.call("GET", regPath, nil, f.token, 500)
}
