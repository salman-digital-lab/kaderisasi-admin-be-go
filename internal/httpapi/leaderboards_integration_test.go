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

func TestCounselingAchievementAndLeaderboard(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	data := objectData(t, f.call("POST", "/v2/members", map[string]string{"name": "Achievement fixture", "gender": "F"}, f.token, 201))
	var u database.Object
	json.Unmarshal(data["user"], &u)
	defer func() {
		if _, err := f.pool.Exec(ctx, "DELETE FROM public_users WHERE id=$1", u.ID("id")); err != nil {
			t.Error(err)
		}
	}()
	counseling := database.Object{}
	counseling.Set("user_id", u.ID("id"))
	counseling.Set("problem_description", "Synthetic fixture")
	counseling.Set("counselor_id", f.adminID)
	counseling, err := f.queries().Insert(ctx, "ruang_curhats", counseling)
	if err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/v2/ruang-curhat?name=Achievement&gender=F&status=0&admin_display_name=Contract", nil, f.token, 200)
	cp := fmt.Sprintf("/v2/ruang-curhat/%d", counseling.ID("id"))
	f.call("GET", cp, nil, f.token, 200)
	f.call("PUT", cp, map[string]interface{}{"status": 1, "additional_notes": "Synthetic notes"}, f.token, 200)
	counselorID := f.admin("konselor")
	counselorToken := f.tokenFor(counselorID)
	f.call("GET", cp, nil, counselorToken, 200)
	options := f.call("GET", "/v2/ruang-curhat/counselors", nil, counselorToken, 200)
	var optionRows []database.Object
	if err = json.Unmarshal(options["data"], &optionRows); err != nil {
		t.Fatal(err)
	}
	foundCounselor := false
	for _, option := range optionRows {
		if option.ID("id") == counselorID {
			foundCounselor = true
		}
		if _, exposesRole := option["role_code"]; exposesRole {
			t.Fatal("counselor options expose role codes")
		}
	}
	if !foundCounselor {
		t.Fatal("active counselor missing from counseling-scoped options")
	}
	f.call("PUT", cp, map[string]interface{}{"counselor_id": counselorID}, counselorToken, 200)
	ineligibleID := f.admin("member_manager")
	f.call("PUT", cp, map[string]interface{}{"counselor_id": ineligibleID}, counselorToken, 422)
	f.call("GET", fmt.Sprintf("/v2/profiles/user/%d", u.ID("id")), nil, counselorToken, 403)
	f.call("GET", "/v2/admin-users", nil, counselorToken, 403)
	a := database.Object{}
	a.Set("user_id", u.ID("id"))
	a.Set("name", "Synthetic award")
	a.Set("type", 2)
	a.Set("score", 10)
	a.Set("achievement_date", "2026-02-28")
	a, err = f.queries().Insert(ctx, "achievements", a)
	if err != nil {
		t.Fatal(err)
	}
	ap := fmt.Sprintf("/v2/achievements/%d", a.ID("id"))
	f.call("GET", ap, nil, f.token, 200)
	f.call("GET", "/v2/achievements?name=Achievement&type=2&status=0&sort_by=achievement_date&sort_order=asc", nil, f.token, 200)
	f.call("PUT", ap, map[string]interface{}{"description": "Synthetic description", "score": 15}, f.token, 200)
	f.call("PUT", ap+"/approve-reject", map[string]interface{}{"score": 20}, f.token, 400)
	approved := objectData(t, f.call("PUT", ap+"/approve-reject", map[string]interface{}{"status": 1, "score": 20}, f.token, 200))
	if approved.ID("approver_id") != f.adminID || approved.String("approved_at") == "" {
		t.Fatal("approval attribution")
	}
	var monthly, total int
	var month string
	if err = f.pool.QueryRow(ctx, "SELECT score,month::text FROM monthly_leaderboards WHERE user_id=$1", u.ID("id")).Scan(&monthly, &month); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, "SELECT score FROM lifetime_leaderboards WHERE user_id=$1", u.ID("id")).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if monthly != 20 || total != 20 || month != "2026-02-01" {
		t.Fatal("leaderboard effects", monthly, total, month)
	}
	f.call("GET", "/v2/leaderboards/monthly?month=2&year=2026&name=Achievement", nil, f.token, 200)
	f.call("GET", "/v2/leaderboards/monthly?year=2026", nil, f.token, 200)
	f.call("GET", "/v2/leaderboards/lifetime?name=Achievement", nil, f.token, 200)
	// Legacy review adds the approved score on every approval, and rejection
	// leaves accumulated scores unchanged. Differential tests must preserve it.
	f.call("PUT", ap+"/approve-reject", map[string]interface{}{"status": 1, "score": 20}, f.token, 200)
	f.call("PUT", ap+"/approve-reject", map[string]interface{}{"status": 2, "remark": "Synthetic rejection"}, f.token, 200)
	if err = f.pool.QueryRow(ctx, "SELECT score FROM lifetime_leaderboards WHERE user_id=$1", u.ID("id")).Scan(&total); err != nil || total != 40 {
		t.Fatal("legacy repeated-review effects", total, err)
	}
	r := httptest.NewRequest("GET", "/v2/achievements/export", nil)
	r.Header.Set("Authorization", "Bearer "+f.token)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("achievement export %d %s", w.Code, w.Body)
	}
	book, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows("Achievements")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || len(rows[0]) != 14 || rows[1][2] != "Achievement fixture" || rows[1][4] != "Akademik" || rows[1][6] != "Ditolak" || rows[1][7] != "28 February 2026" {
		t.Fatalf("achievement export content %v", rows)
	}
	f.call("GET", "/v2/achievements/2147483647", nil, f.token, 404)
	f.call("PUT", "/v2/achievements/2147483647", map[string]interface{}{"name": "missing"}, f.token, 404)
}
