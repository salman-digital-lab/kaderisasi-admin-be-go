//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/xuri/excelize/v2"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistrationWorkflowAndExport(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	users := []int32{}
	activities := []int32{}
	defer func() {
		for _, target := range []struct {
			table string
			ids   []int32
		}{{"activities", activities}, {"public_users", users}} {
			if _, err := f.pool.Exec(ctx, "DELETE FROM "+target.table+" WHERE id=ANY($1::int[])", target.ids); err != nil {
				t.Error(err)
			}
		}
	}()
	uuid, _ := auth.UUID()
	email := uuid + "@example.test"
	memberData := objectData(t, f.call("POST", "/v2/members", map[string]interface{}{"name": "Registrant fixture", "email": email}, f.token, 201))
	var user, profile database.Object
	json.Unmarshal(memberData["user"], &user)
	json.Unmarshal(memberData["profile"], &profile)
	users = append(users, user.ID("id"))
	config := map[string]interface{}{"custom_selection_status": []string{}, "mandatory_profile_data": []interface{}{}, "additional_questionnaire": []interface{}{map[string]interface{}{"name": "motivation", "label": "Motivasi", "type": "text"}}}
	a := objectData(t, f.call("POST", "/v2/activities", map[string]interface{}{"name": "Registration fixture", "activity_type": 2, "badge": "Fixture SSC", "additional_config": config}, f.token, 200))
	activities = append(activities, a.ID("id"))
	path := fmt.Sprintf("/v2/activities/%d/registrations", a.ID("id"))
	payload := map[string]interface{}{"user_id": profile.ID("id"), "questionnaire_answer": map[string]interface{}{"motivation": "Learn"}}
	reg := objectData(t, f.call("POST", path, payload, f.token, 200))
	id := reg.ID("id")
	f.call("POST", path, payload, f.token, 409)
	f.call("GET", fmt.Sprintf("/v2/activity-registrations/%d", id), nil, f.token, 200)
	f.call("GET", fmt.Sprintf("/v2/activity-registrations/user/%d", user.ID("id")), nil, f.token, 200)
	guest := database.Object{}
	guest.Set("activity_id", a.ID("id"))
	guest.Set("guest_data", map[string]interface{}{"name": "Guest fixture", "email": "guest-" + email, "whatsapp": "081234567890", "country": "Indonesia", "current_education": map[string]interface{}{"institution": "Guest University", "intake_year": 2024}})
	guest.Set("status", "TERDAFTAR")
	guest.Set("questionnaire_answer", map[string]interface{}{"motivation": []string{"Learn", "Share"}})
	guestReg, err := f.queries().Insert(ctx, "activity_registrations", guest)
	if err != nil {
		t.Fatal(err)
	}
	page := objectData(t, f.call("GET", path+"?search=Guest&sort_by=name&sort_order=asc", nil, f.token, 200))
	var matches []database.Object
	json.Unmarshal(page["data"], &matches)
	if len(matches) != 1 || matches[0].String("name") != "Guest fixture" {
		t.Fatalf("guest search %s", page["data"])
	}
	f.call("PUT", "/v2/activity-registrations", map[string]interface{}{"registrations_id": []int32{id, guestReg.ID("id")}, "status": "LULUS KEGIATAN"}, f.token, 200)
	f.call("PUT", "/v2/activity-registrations", map[string]interface{}{"registrations_id": []int32{id}, "status": "LULUS KEGIATAN"}, f.token, 200)
	var level, badgeCount int
	if err = f.pool.QueryRow(ctx, "SELECT level,jsonb_array_length(badges) FROM profiles WHERE id=$1", profile.ID("id")).Scan(&level, &badgeCount); err != nil || level != 3 || badgeCount != 1 {
		t.Fatal("idempotent profile upgrade", level, badgeCount, err)
	}
	f.call("PUT", path+"/status-by-email", map[string]interface{}{"emails": []string{email}, "status": "DITERIMA"}, f.token, 200)
	f.call("PUT", path+"/status-by-email", map[string]interface{}{"emails": []string{"missing-" + email}, "status": "DITERIMA"}, f.token, 404)
	f.call("PUT", path, map[string]string{"current_status": "DITERIMA", "new_status": "TERDAFTAR"}, f.token, 200)
	stats := objectData(t, f.call("GET", path+"/statistics", nil, f.token, 200))
	if stats.ID("total") != 2 {
		t.Fatalf("statistics %v", stats)
	}
	r := httptest.NewRequest("GET", fmt.Sprintf("/v2/activities/%d/registrations-export", a.ID("id")), nil)
	r.Header.Set("Authorization", "Bearer "+f.token)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("export %d %s", w.Code, w.Body)
	}
	if !strings.Contains(w.Header().Get("Content-Disposition"), "Registration-fixture.xlsx") {
		t.Fatal("export filename")
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
	if len(rows) != 3 || len(rows[0]) != 31 || rows[0][30] != "Motivasi" {
		t.Fatalf("export columns %#v", rows)
	}
	foundMember, foundGuest := false, false
	for _, row := range rows[1:] {
		switch row[1] {
		case "Registrant fixture":
			foundMember = row[3] == email && row[27] == "AKTIVIS" && row[28] == "Fixture SSC" && row[30] == "Learn"
		case "Guest fixture":
			foundGuest = row[20] == "Guest University" && row[23] == "2024" && row[27] == "Tamu" && row[30] == `["Learn","Share"]`
		}
	}
	if !foundMember || !foundGuest {
		t.Fatalf("export values %#v", rows)
	}
	f.call("DELETE", fmt.Sprintf("/v2/activity-registrations/%d", id), nil, f.token, 200)
	f.call("DELETE", fmt.Sprintf("/v2/activity-registrations/%d", id), nil, f.token, 200)
	f.call("GET", "/v2/activity-registrations/2147483647", nil, f.token, 500)
}
func (f *httpFixture) queries() database.JSONQueries { return database.JSONQueries{DB: f.pool} }
