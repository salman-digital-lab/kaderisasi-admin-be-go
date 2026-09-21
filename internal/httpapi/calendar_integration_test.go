//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/calendar"
	"kaderisasi/admin/internal/database"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type calendarCheck struct {
	Owner  string `json:"owner"`
	Label  string `json:"label"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
}
type calendarFixture struct {
	*httpFixture
	checks []calendarCheck
}

func (f *calendarFixture) call(method, path string, body interface{}, token string, status int) database.Object {
	f.t.Helper()
	result := f.httpFixture.call(method, path, body, token, status)
	label := method + " " + path
	if status == 422 || strings.Contains(path, "/bad") {
		label = "invalid-input: " + label
	}
	if status == 404 {
		label = "missing-resource: " + label
	}
	f.checks = append(f.checks, calendarCheck{Owner: "admin", Label: label, Method: method, Path: strings.TrimPrefix(path, "/v2"), Status: status})
	return result
}

func TestCalendarWorkflow(t *testing.T) {
	f := &calendarFixture{httpFixture: newHTTPFixture(t)}
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		raw, err := json.Marshal(f.checks)
		if err != nil {
			t.Error(err)
			return
		}
		if err = os.WriteFile(filepath.Join(os.Getenv("GO_REWRITE_ARTIFACTS"), "calendar-cases.json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	ctx := context.Background()
	ids := []int32{}
	uuid, _ := auth.UUID()
	var activityID int32
	if err := f.pool.QueryRow(ctx, `INSERT INTO activities(name,slug,is_published) VALUES('Private calendar activity',$1,false) RETURNING id`, uuid).Scan(&activityID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := f.pool.Exec(ctx, "DELETE FROM calendar_events WHERE id=ANY($1::int[])", ids); err != nil {
			t.Error(err)
		}
		if _, err := f.pool.Exec(ctx, "DELETE FROM activities WHERE id=$1", activityID); err != nil {
			t.Error(err)
		}
	})
	rangeQuery := "?" + url.Values{"start": {"2026-09-01T00:00:00+07:00"}, "end": {"2026-10-01T00:00:00+07:00"}}.Encode()
	input := map[string]interface{}{"title": "Independent title", "description": "Public description", "location": "Salman", "starts_at": "2026-08-31T00:00:00+07:00", "ends_at": "2026-09-02T00:00:00+07:00", "all_day": true, "activity_id": activityID}
	root := "/v2/admin/calendar-events"
	f.call("GET", "/v2/calendar-events", nil, "", 422)
	f.call("GET", root+rangeQuery, nil, "", 401)
	for _, method := range []string{"POST", "PUT", "DELETE"} {
		path := root
		if method != "POST" {
			path += "/1"
		}
		f.call(method, path, input, "", 401)
	}
	var data calendar.Event
	if err := json.Unmarshal(f.call("POST", root, input, f.token, 201)["data"], &data); err != nil {
		t.Fatal(err)
	}
	id := data.ID
	ids = append(ids, id)
	path := fmt.Sprintf("%s/%d", root, id)
	f.call("GET", path, nil, "", 401)
	unassigned := f.tokenFor(f.admin(""))
	f.call("GET", root+rangeQuery, nil, unassigned, 403)
	f.call("GET", path, nil, unassigned, 403)
	f.call("GET", root, nil, f.token, 422)
	f.call("PUT", path, map[string]interface{}{"title": ""}, f.token, 422)
	f.call("PUT", root+"/2147483647", input, f.token, 404)
	f.call("DELETE", root+"/bad", nil, f.token, 404)
	for _, role := range auth.Roles() {
		token := f.tokenFor(f.admin(role.Code))
		f.call("GET", root+rangeQuery, nil, token, 200)
		f.call("GET", path, nil, token, 200)
		expected := 403
		if role.Code == "admin" || role.Code == "super_admin" {
			expected = 200
		}
		f.call("PUT", path, input, token, expected)
		if expected == 200 {
			var created calendar.Event
			if err := json.Unmarshal(f.call("POST", root, input, token, 201)["data"], &created); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, created.ID)
			f.call("DELETE", fmt.Sprintf("%s/%d", root, created.ID), nil, token, 200)
		}
		if expected == 403 {
			f.call("POST", root, input, token, 403)
			f.call("DELETE", path, nil, token, 403)
		}
	}
	combined := f.admin("konselor")
	if _, err := f.pool.Exec(ctx, "UPDATE admin_users SET additional_role_codes=ARRAY['admin'] WHERE id=$1", combined); err != nil {
		t.Fatal(err)
	}
	f.call("PUT", path, input, f.tokenFor(combined), 200)
	find := func() calendar.Event {
		t.Helper()
		var rows []calendar.Event
		if err := json.Unmarshal(f.call("GET", "/v2/calendar-events"+rangeQuery, nil, "", 200)["data"], &rows); err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			if row.ID == id {
				return row
			}
		}
		t.Fatal("overlapping calendar event missing")
		return calendar.Event{}
	}
	if find().Activity != nil {
		t.Fatal("draft activity leaked")
	}
	if _, err := f.pool.Exec(ctx, "UPDATE activities SET is_published=true,name='Changed activity' WHERE id=$1", activityID); err != nil {
		t.Fatal(err)
	}
	event := find()
	if event.Activity == nil || event.Title != "Independent title" {
		t.Fatal("activity reference or independent title incorrect")
	}
	emptyRange := "?" + url.Values{"start": {"2026-09-02T00:00:00+07:00"}, "end": {"2026-09-03T00:00:00+07:00"}}.Encode()
	var rows []calendar.Event
	if err := json.Unmarshal(f.call("GET", "/v2/calendar-events"+emptyRange, nil, "", 200)["data"], &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ID == id {
			t.Fatal("exclusive end leaked into following date")
		}
	}
	input["title"] = "Edited title"
	input["activity_id"] = nil
	f.call("PUT", path, input, f.token, 200)
	if find().Activity != nil {
		t.Fatal("unlink failed")
	}
	input["activity_id"] = activityID
	f.call("PUT", path, input, f.token, 200)
	if _, err := f.pool.Exec(ctx, "DELETE FROM activities WHERE id=$1", activityID); err != nil {
		t.Fatal(err)
	}
	if find().Activity != nil {
		t.Fatal("deleted activity link survived")
	}
	input["activity_id"] = activityID
	f.call("POST", root, input, f.token, 422)
	input["activity_id"] = nil
	input["ends_at"] = input["starts_at"]
	f.call("POST", root, input, f.token, 422)
	f.call("DELETE", path, nil, f.token, 200)
	f.call("GET", path, nil, f.token, 404)
	f.call("DELETE", path, nil, f.token, 404)
	f.call("GET", root+"/bad", nil, f.token, 404)
}
