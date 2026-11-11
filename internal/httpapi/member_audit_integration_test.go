//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
)

func TestMemberAuditTrail(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	var userID, profileID int32
	t.Cleanup(func() {
		for _, statement := range []string{
			"DELETE FROM member_audit_logs WHERE user_id=$1 OR profile_id=$2",
			"DELETE FROM profiles WHERE user_id=$1 OR id=$2",
			"DELETE FROM public_users WHERE id=$1 AND $2::int IS NOT NULL",
		} {
			if _, err := f.pool.Exec(context.Background(), statement, userID, profileID); err != nil {
				t.Error(err)
			}
		}
	})
	uid, _ := auth.UUID()
	email := uid + "@example.test"

	created := objectData(t, f.call("POST", "/v2/members", map[string]interface{}{"name": "Audit fixture", "gender": "M"}, f.token, 201))
	var user, profile database.Object
	json.Unmarshal(created["user"], &user)
	json.Unmarshal(created["profile"], &profile)
	userID, profileID = user.ID("id"), profile.ID("id")
	profilePath := fmt.Sprintf("/v2/profiles/%d", profileID)
	trailPath := profilePath + "/audit-logs"

	type entry struct {
		Action    string                                `json:"action"`
		Source    string                                `json:"source"`
		ActorID   *int32                                `json:"actor_admin_user_id"`
		ActorName string                                `json:"actor_name"`
		CreatedAt *string                               `json:"created_at"`
		Changes   map[string]map[string]json.RawMessage `json:"changes"`
	}
	trail := func(query string, token string) ([]entry, database.Object) {
		data := objectData(t, f.call("GET", trailPath+query, nil, token, 200))
		var rows []entry
		if err := json.Unmarshal(data["data"], &rows); err != nil {
			t.Fatal(err)
		}
		var meta database.Object
		json.Unmarshal(data["meta"], &meta)
		return rows, meta
	}
	latest := func(action string) entry {
		t.Helper()
		rows, _ := trail("", f.token)
		if len(rows) == 0 || rows[0].Action != action {
			t.Fatalf("latest entry is not %s: %+v", action, rows)
		}
		return rows[0]
	}

	first := latest("created")
	if string(first.Changes["name"]["after"]) != `"Audit fixture"` || string(first.Changes["account_status"]["after"]) != `"no_account"` ||
		first.ActorID == nil || *first.ActorID != f.adminID || first.ActorName != "Contract fixture" || first.Source != "admin" || first.CreatedAt == nil {
		t.Fatalf("created entry %+v", first)
	}

	// Only the changed fields are recorded, with the replaced value.
	f.call("PUT", profilePath, map[string]interface{}{"name": "Audit renamed", "gender": "M", "whatsapp": "0811000"}, f.token, 200)
	updated := latest("profile_updated")
	if len(updated.Changes) != 2 || string(updated.Changes["name"]["before"]) != `"Audit fixture"` || string(updated.Changes["name"]["after"]) != `"Audit renamed"` ||
		string(updated.Changes["whatsapp"]["before"]) != "null" {
		t.Fatalf("update entry %+v", updated.Changes)
	}
	// A save that changes nothing leaves no entry.
	f.call("PUT", profilePath, map[string]interface{}{"name": "Audit renamed"}, f.token, 200)
	if rows, _ := trail("", f.token); len(rows) != 2 {
		t.Fatalf("unchanged save was recorded: %d entries", len(rows))
	}

	f.call("PUT", profilePath+"/regional-assignment", map[string]interface{}{"alumni_regional_assignment": []string{"Bandung"}}, f.token, 200)
	regional := latest("regional_assignment_updated")
	if len(regional.Changes) != 1 || string(regional.Changes["extra_data.alumni_regional_assignment"]["after"]) != `["Bandung"]` {
		t.Fatalf("regional entry %+v", regional.Changes)
	}

	f.call("POST", fmt.Sprintf("/v2/members/%d/generate-account", userID), map[string]string{"email": email, "password": "Fixture-generated-2026!"}, f.token, 200)
	generated := latest("account_generated")
	if string(generated.Changes["email"]["after"]) != `"`+email+`"` || string(generated.Changes["account_status"]["after"]) != `"active"` || string(generated.Changes["password"]["changed"]) != "true" {
		t.Fatalf("account entry %+v", generated.Changes)
	}

	f.call("PUT", fmt.Sprintf("/v2/profiles/auth/%d", userID), map[string]string{"email": "new-" + email, "password": "Fixture-changed-2026!"}, f.token, 200)
	credentials := latest("credentials_updated")
	if string(credentials.Changes["email"]["before"]) != `"`+email+`"` || string(credentials.Changes["password"]["changed"]) != "true" {
		t.Fatalf("credential entry %+v", credentials.Changes)
	}
	f.call("PUT", profilePath, map[string]interface{}{"password": "Fixture-reset-2026!"}, f.token, 200)
	if reset := latest("credentials_updated"); len(reset.Changes) != 1 || string(reset.Changes["password"]["changed"]) != "true" {
		t.Fatalf("password reset entry %+v", reset.Changes)
	}

	// No password or hash is ever stored in the trail.
	var stored string
	if err := f.pool.QueryRow(ctx, "SELECT coalesce(string_agg(changes::text,' '),'') FROM member_audit_logs WHERE user_id=$1", userID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	var hash string
	if err := f.pool.QueryRow(ctx, "SELECT password FROM public_users WHERE id=$1", userID).Scan(&hash); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored, "Fixture-") || strings.Contains(stored, hash) || strings.Contains(stored, "$scrypt") || strings.Contains(stored, "$argon") {
		t.Fatalf("trail contains a secret: %s", stored)
	}

	// Newest first, paginated.
	rows, meta := trail("?per_page=2&page=2", f.token)
	if len(rows) != 2 || rows[0].Action != "account_generated" || rows[1].Action != "regional_assignment_updated" || meta.ID("total") != 6 {
		t.Fatalf("page 2: %+v %v", rows, meta)
	}
	f.call("GET", trailPath+"?page=0", nil, f.token, 422)
	f.call("GET", "/v2/profiles/abc/audit-logs", nil, f.token, 404)

	// The trail is narrower than the member page, and has no write route.
	f.call("GET", trailPath, nil, "", 401)
	for _, role := range []string{"alumni_manager", "activity_manager", "club_manager"} {
		f.call("GET", trailPath, nil, f.tokenFor(f.admin(role)), 403)
	}
	if rows, _ := trail("", f.tokenFor(f.admin("admin"))); len(rows) != 6 {
		t.Fatalf("Asisten Manager Program sees %d entries", len(rows))
	}
	for _, method := range []string{"POST", "PUT", "DELETE"} {
		f.call(method, trailPath, map[string]string{}, f.token, 404)
	}

	// A deleted member keeps its trail, readable by the former profile id.
	f.call("DELETE", profilePath, nil, f.token, 200)
	// Deleting a missing profile keeps its legacy 500 and records nothing.
	f.call("DELETE", profilePath, nil, f.token, 500)
	deleted := latest("deleted")
	if string(deleted.Changes["name"]["before"]) != `"Audit renamed"` || string(deleted.Changes["name"]["after"]) != "null" {
		t.Fatalf("deleted entry %+v", deleted.Changes)
	}
	if rows, _ := trail("", f.token); len(rows) != 7 {
		t.Fatalf("trail after deletion has %d entries", len(rows))
	}
}
