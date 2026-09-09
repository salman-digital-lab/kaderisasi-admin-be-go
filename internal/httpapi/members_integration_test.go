//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"testing"
)

func TestMemberProfileWorkflow(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	users := []int32{}
	defer func() {
		if _, err := f.pool.Exec(ctx, "DELETE FROM public_users WHERE id=ANY($1::int[])", users); err != nil {
			t.Error(err)
		}
	}()
	f.call("POST", "/v2/members", map[string]string{}, f.token, 500)
	f.call("POST", "/v2/members", map[string]string{"name": "Fixture"}, "", 401)
	uid, _ := auth.UUID()
	email := uid + "@example.test"
	create := func(body map[string]interface{}) (database.Object, database.Object) {
		data := objectData(t, f.call("POST", "/v2/members", body, f.token, 201))
		var user, profile database.Object
		json.Unmarshal(data["user"], &user)
		json.Unmarshal(data["profile"], &profile)
		users = append(users, user.ID("id"))
		return user, profile
	}
	user, profile := create(map[string]interface{}{"name": "Fixture no account", "gender": "M", "birth_date": "1999-03-15"})
	if user.String("account_status") != "no_account" || user.String("member_id") != fmt.Sprintf("%08d", user.ID("id")) || user.Has("password") {
		t.Fatalf("user contract %v", user)
	}
	var storedBadges []byte
	if err := f.pool.QueryRow(ctx, "SELECT badges FROM profiles WHERE id=$1", profile.ID("id")).Scan(&storedBadges); err != nil || string(storedBadges) != "[]" {
		t.Fatalf("stored badge default: %s %v", storedBadges, err)
	}
	path := fmt.Sprintf("/v2/members/%d/generate-account", user.ID("id"))
	f.call("POST", path, map[string]string{"email": email, "password": "Fixture-generated-2026!"}, f.token, 200)
	f.call("POST", path, map[string]string{"email": email, "password": "Fixture-generated-2026!"}, f.token, 400)
	f.call("POST", "/v2/members", map[string]interface{}{"name": "Duplicate", "email": email}, f.token, 409)
	f.call("POST", "/v2/members", map[string]interface{}{"name": "Duplicate ID", "member_id": user.String("member_id")}, f.token, 409)
	f.call("POST", "/v2/members/2147483647/generate-account", map[string]string{"email": email, "password": "Fixture-generated-2026!"}, f.token, 404)
	profilePath := fmt.Sprintf("/v2/profiles/%d", profile.ID("id"))
	f.call("PUT", profilePath, map[string]interface{}{"name": "Updated member", "badges": []string{"LMD"}, "education_history": []interface{}{map[string]interface{}{"degree": "bachelor", "institution": "Fixture University", "faculty": "Science", "major": "Physics", "intake_year": 2020}}, "extra_data": map[string]interface{}{"preferred_name": "Fixture"}}, f.token, 200)
	f.call("PUT", profilePath+"/regional-assignment", map[string]interface{}{"alumni_regional_assignment": []string{"Bandung"}}, f.token, 200)
	f.call("PUT", profilePath, map[string]interface{}{"extra_data": map[string]interface{}{"salman_activity_history": []string{"Fixture activity"}}, "password": "Fixture-reset-2026!"}, f.token, 200)
	got := objectData(t, f.call("GET", profilePath, nil, f.token, 200))
	var profiles []database.Object
	json.Unmarshal(got["profile"], &profiles)
	if len(profiles) != 1 {
		t.Fatalf("profile array %s", got["profile"])
	}
	var extra database.Object
	json.Unmarshal(profiles[0]["extra_data"], &extra)
	if extra.String("preferred_name") != "Fixture" || !extra.Has("alumni_regional_assignment") || !extra.Has("salman_activity_history") {
		t.Fatalf("lost extra fields: %s", profiles[0]["extra_data"])
	}
	var hash string
	if err := f.pool.QueryRow(ctx, "SELECT password FROM public_users WHERE id=$1", user.ID("id")).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if !auth.VerifyPassword(hash, "Fixture-reset-2026!") {
		t.Fatal("profile password reset")
	}
	f.call("GET", fmt.Sprintf("/v2/profiles/user/%d", user.ID("id")), nil, f.token, 200)
	page := objectData(t, f.call("GET", "/v2/profiles?badge=LMD&education_institution=Fixture&member_id="+user.String("member_id"), nil, f.token, 200))
	var rows []database.Object
	json.Unmarshal(page["data"], &rows)
	if len(rows) != 1 {
		t.Fatalf("filtered profiles: %s", page["data"])
	}
	f.call("PUT", "/v2/profiles/2147483647/regional-assignment", map[string]interface{}{"alumni_regional_assignment": []string{}}, f.token, 404)
	user2, _ := create(map[string]interface{}{"name": "Active fixture", "email": uid + "-2@example.test", "password": "Fixture-active-2026!"})
	if user2.String("account_status") != "active" {
		t.Fatal("active account")
	}
	f.call("PUT", fmt.Sprintf("/v2/profiles/auth/%d", user2.ID("id")), map[string]string{"email": uid + "-updated@example.test", "password": "Fixture-change-2026!"}, f.token, 200)
	f.call("DELETE", profilePath, nil, f.token, 200)
	f.call("DELETE", profilePath, nil, f.token, 500)
}
