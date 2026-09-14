//go:build integration

package httpapi

import (
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/auth"
	"testing"
)

func TestAdminMultipleRoles(t *testing.T) {
	f := newHTTPFixture(t)
	uuid, err := auth.UUID()
	if err != nil {
		t.Fatal(err)
	}
	created := objectData(t, f.call("POST", "/v2/admin-users", map[string]interface{}{
		"displayName": "Multiple roles", "email": uuid + "@example.test", "password": "Fixture-password-2026!",
		"role_codes": []string{"activity_manager", "konselor", "activity_manager"},
	}, f.token, 201))
	id := created.ID("id")
	f.admins = append(f.admins, id)
	var codes []string
	if err := json.Unmarshal(created["role_codes"], &codes); err != nil || len(codes) != 2 {
		t.Fatalf("role codes: %s, %v", created["role_codes"], err)
	}
	token := f.tokenFor(id)
	f.call("GET", "/v2/activities", nil, token, 200)
	f.call("GET", "/v2/ruang-curhat", nil, token, 200)
	f.call("GET", "/v2/admin-users", nil, token, 403)
	page := objectData(t, f.call("GET", "/v2/admin-users?search="+uuid+"&role_code=konselor", nil, f.token, 200))
	var users []json.RawMessage
	if err := json.Unmarshal(page["data"], &users); err != nil || len(users) != 1 {
		t.Fatal("secondary role filter failed")
	}
	path := fmt.Sprintf("/v2/admin-users/%d", id)
	f.call("PUT", path, map[string]interface{}{"role_codes": []string{"konselor"}}, f.token, 200)
	f.call("GET", "/v2/activities", nil, token, 403)
	f.call("GET", "/v2/ruang-curhat", nil, token, 200)
	f.call("PUT", path, map[string]interface{}{"displayName": "Retain roles"}, f.token, 200)
	if !auth.ForUser(mustUser(t, f.pool, id)).Allows("counseling.manage") {
		t.Fatal("omitted roles cleared access")
	}
	f.call("PUT", path, map[string]interface{}{"role_codes": []string{}}, f.token, 200)
	f.call("GET", "/v2/ruang-curhat", nil, token, 403)
	superPath := fmt.Sprintf("/v2/admin-users/%d", f.adminID)
	f.call("PUT", superPath, map[string]interface{}{"role_codes": []string{"konselor", "super_admin"}}, f.token, 200)
	f.call("PUT", superPath, map[string]interface{}{"role_codes": []string{"konselor"}}, f.token, 409)
	f.call("PUT", superPath, map[string]interface{}{"role_code": "super_admin"}, f.token, 200)
}
