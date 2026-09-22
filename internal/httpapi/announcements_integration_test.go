//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/announcement"
	"kaderisasi/admin/internal/dbgen"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestAnnouncementWorkflow(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	q := dbgen.New(f.pool)
	svc := announcement.Service{Pool: f.pool}
	memberIDs := []int32{}
	announcementIDs := []int32{}
	var activityID, clubID int32
	t.Cleanup(func() {
		for _, item := range []struct {
			sql   string
			value interface{}
		}{
			{"DELETE FROM announcements WHERE id=ANY($1::integer[])", announcementIDs},
			{"DELETE FROM activity_registrations WHERE activity_id=$1", activityID}, {"DELETE FROM activities WHERE id=$1", activityID},
			{"DELETE FROM club_registrations WHERE club_id=$1", clubID}, {"DELETE FROM clubs WHERE id=$1", clubID},
			{"DELETE FROM public_users WHERE id=ANY($1::integer[])", memberIDs},
		} {
			if _, e := f.pool.Exec(ctx, item.sql, item.value); e != nil {
				t.Error(e)
			}
		}
	})
	member := func(status string) int32 {
		var id int32
		if e := f.pool.QueryRow(ctx, "INSERT INTO public_users(email,account_status,created_at) VALUES($1,$2,now()) RETURNING id", fmt.Sprintf("announcement-%d@example.test", len(memberIDs)), status).Scan(&id); e != nil {
			t.Fatal(e)
		}
		memberIDs = append(memberIDs, id)
		return id
	}
	active := member("active")
	inactive := member("no_account")
	otherMember := member("active")
	if e := f.pool.QueryRow(ctx, "INSERT INTO activities(name,slug) VALUES('Announcement fixture','announcement-fixture') RETURNING id").Scan(&activityID); e != nil {
		t.Fatal(e)
	}
	if e := f.pool.QueryRow(ctx, "INSERT INTO clubs(name) VALUES('Announcement club') RETURNING id").Scan(&clubID); e != nil {
		t.Fatal(e)
	}
	if _, e := f.pool.Exec(ctx, "INSERT INTO activity_registrations(activity_id,user_id,status) VALUES($1,$2,'CUSTOM'),($1,$3,'TERDAFTAR')", activityID, active, otherMember); e != nil {
		t.Fatal(e)
	}
	if _, e := f.pool.Exec(ctx, "INSERT INTO club_registrations(club_id,member_id,status) VALUES($1,$2,'APPROVED')", clubID, active); e != nil {
		t.Fatal(e)
	}
	roleAdmin := f.admin("activity_manager")
	inactiveAdmin := f.admin("admin")
	outsider := f.admin("")
	if _, e := f.pool.Exec(ctx, "UPDATE admin_users SET additional_role_codes=ARRAY['club_manager'] WHERE id=$1", roleAdmin); e != nil {
		t.Fatal(e)
	}
	if _, e := f.pool.Exec(ctx, "UPDATE admin_users SET is_active=false WHERE id=$1", inactiveAdmin); e != nil {
		t.Fatal(e)
	}
	input := announcement.Input{Title: "Fixture announcement", Body: "Plain <script>text</script>", Audience: announcement.Audience{MemberIDs: []int32{active, inactive}, ActivityIDs: []int32{activityID}, ActivityStatuses: []string{"CUSTOM"}, ClubIDs: []int32{clubID}, AdminIDs: []int32{f.adminID, roleAdmin, inactiveAdmin}, RoleCodes: []string{"club_manager"}}, Version: 1}
	create := func(in announcement.Input) announcement.Record {
		t.Helper()
		var row announcement.Record
		if e := json.Unmarshal(f.call("POST", "/v2/announcements", in, f.token, 201)["data"], &row); e != nil {
			t.Fatal(e)
		}
		announcementIDs = append(announcementIDs, row.ID)
		return row
	}
	f.call("GET", "/v2/announcements", nil, "", 401)
	for _, role := range []string{"admin", "activity_manager", "club_manager", "konselor", "achievement_manager"} {
		if _, e := f.pool.Exec(ctx, "UPDATE admin_users SET role_code=$1 WHERE id=$2", role, outsider); e != nil {
			t.Fatal(e)
		}
		token := f.tokenFor(outsider)
		f.call("POST", "/v2/announcements", input, token, 403)
		f.call("GET", "/v2/notifications", nil, token, 200)
	}
	if _, e := f.pool.Exec(ctx, "UPDATE admin_users SET role_code=NULL WHERE id=$1", outsider); e != nil {
		t.Fatal(e)
	}
	row := create(input)
	path := fmt.Sprintf("/v2/announcements/%d", row.ID)
	var preview announcement.Preview
	if e := json.Unmarshal(f.call("POST", path+"/preview", nil, f.token, 200)["data"], &preview); e != nil {
		t.Fatal(e)
	}
	if preview.Eligible != 3 || preview.Excluded != 2 || preview.Members != 1 || preview.Admins != 2 {
		t.Fatal("recipient union", preview)
	}
	f.call("PATCH", path, input, f.token, 200)
	f.call("PATCH", path, input, f.token, 409)
	f.call("POST", path+"/publish", map[string]int{"version": 1}, f.token, 409)
	var wg sync.WaitGroup
	failures := make(chan error, 4)
	for range 4 {
		wg.Go(func() { _, e := svc.Publish(ctx, row.ID, f.adminID, 2); failures <- e })
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		if e != nil {
			t.Fatal(e)
		}
	}
	published, e := q.AnnouncementGet(ctx, row.ID)
	if e != nil || published.RecipientCount != 3 {
		t.Fatal("publication", published, e)
	}
	var count int
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM announcement_recipients WHERE announcement_id=$1", row.ID).Scan(&count); e != nil || count != 3 {
		t.Fatal("duplicate recipients", count, e)
	}
	f.call("PATCH", path, input, f.token, 409)
	f.call("DELETE", path, map[string]int{"version": 3}, f.token, 409)
	if _, e = f.pool.Exec(ctx, "UPDATE public_users SET account_status='active' WHERE id=$1", inactive); e != nil {
		t.Fatal(e)
	}
	f.call("POST", path+"/publish", map[string]int{"version": 2}, f.token, 200)
	var notificationID int32
	if e = f.pool.QueryRow(ctx, "SELECT id FROM announcement_recipients WHERE announcement_id=$1 AND admin_user_id=$2", row.ID, f.adminID).Scan(&notificationID); e != nil {
		t.Fatal(e)
	}
	notificationPath := fmt.Sprintf("/v2/notifications/%d", notificationID)
	outsiderToken := f.tokenFor(outsider)
	f.call("GET", notificationPath, nil, outsiderToken, 404)
	f.call("PUT", notificationPath+"/read", nil, outsiderToken, 404)
	inbox := objectData(t, f.call("GET", "/v2/notifications", nil, f.token, 200))
	cutoff := inbox.String("cutoff")
	newer := create(input)
	if _, e = svc.Publish(ctx, newer.ID, f.adminID, 1); e != nil {
		t.Fatal(e)
	}
	f.call("PUT", "/v2/notifications/read-all", map[string]string{"cutoff": cutoff}, f.token, 200)
	if data := objectData(t, f.call("GET", "/v2/notifications/unread-count", nil, f.token, 200)); data.ID("unread") != 1 {
		t.Fatal("cutoff marked later publication", data)
	}
	read := objectData(t, f.call("PUT", notificationPath+"/read", nil, f.token, 200))
	again := objectData(t, f.call("PUT", notificationPath+"/read", nil, f.token, 200))
	if string(read["read_at"]) != string(again["read_at"]) {
		t.Fatal("read timestamp changed")
	}
	f.call("POST", path+"/withdraw", nil, f.token, 200)
	f.call("GET", notificationPath, nil, f.token, 404)
	f.call("PUT", notificationPath+"/read", nil, f.token, 404)
	empty := input
	empty.Audience = announcement.Audience{}
	emptyRow := create(empty)
	f.call("POST", fmt.Sprintf("/v2/announcements/%d/publish", emptyRow.ID), map[string]int{"version": 1}, f.token, 422)
	if draft, e := q.AnnouncementGet(ctx, emptyRow.ID); e != nil || draft.State != "draft" {
		t.Fatal("failed send changed draft", e)
	}
	// Publication rolls back if either recipient insertion fails.
	rollback := create(input)
	if _, e = f.pool.Exec(ctx, fmt.Sprintf("ALTER TABLE announcement_recipients ADD CONSTRAINT announcement_test_failure CHECK (announcement_id <> %d OR admin_user_id IS NULL)", rollback.ID)); e != nil {
		t.Fatal(e)
	}
	_, sendErr := svc.Publish(ctx, rollback.ID, f.adminID, 1)
	if _, e = f.pool.Exec(ctx, "ALTER TABLE announcement_recipients DROP CONSTRAINT announcement_test_failure"); e != nil {
		t.Fatal(e)
	}
	if sendErr == nil {
		t.Fatal("expected insertion failure")
	}
	if e = f.pool.QueryRow(ctx, "SELECT count(*) FROM announcement_recipients WHERE announcement_id=$1", rollback.ID).Scan(&count); e != nil || count != 0 {
		t.Fatal("partial publication", count, e)
	}
	// Stable pagination when publication times collide.
	for range 22 {
		item := create(input)
		if _, e = svc.Publish(ctx, item.ID, f.adminID, 1); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = f.pool.Exec(ctx, "UPDATE announcements SET published_at='2026-01-01T00:00:00Z' WHERE id=ANY($1::integer[]) AND state='published'", announcementIDs); e != nil {
		t.Fatal(e)
	}
	seen := map[int32]bool{}
	cursor := ""
	for {
		data := objectData(t, f.call("GET", "/v2/notifications?cursor="+url.QueryEscape(cursor), nil, f.token, 200))
		var items []dbgen.AdminNotificationListRow
		if e = json.Unmarshal(data["items"], &items); e != nil {
			t.Fatal(e)
		}
		for _, item := range items {
			if seen[item.ID] {
				t.Fatal("duplicate page item")
			}
			seen[item.ID] = true
		}
		cursor = data.String("next_cursor")
		if cursor == "" {
			break
		}
	}
	if len(seen) != 23 {
		t.Fatal("pagination lost items", len(seen))
	}
	f.call("GET", "/v2/notifications?cursor=invalid", nil, f.token, 422)
	f.call("PUT", "/v2/notifications/read-all", map[string]string{"cutoff": time.Now().Add(time.Hour).Format(time.RFC3339Nano)}, f.token, 422)
}
