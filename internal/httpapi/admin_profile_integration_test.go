//go:build integration

package httpapi

import (
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/database"
	"strings"
	"testing"
)

func TestAdminProfileAndFilters(t *testing.T) {
	f := newHTTPFixture(t)
	id := f.admin("konselor")
	token := f.tokenFor(id)
	path := fmt.Sprintf("/v2/admin-users/%d", id)
	f.call("PUT", "/v2/auth/profile", map[string]string{"displayName": "  Nama Konselor  ", "role_code": "super_admin"}, token, 200)
	user := mustUser(t, f.pool, id)
	if user.DisplayName == nil || *user.DisplayName != "Nama Konselor" || user.RoleCode == nil || *user.RoleCode != "konselor" {
		t.Fatal("profile update must trim name without granting access")
	}
	f.call("PUT", path, map[string]string{"displayName": "Forbidden"}, token, 403)
	for _, name := range []string{"", "   ", strings.Repeat("a", 256)} {
		f.call("PUT", "/v2/auth/profile", map[string]string{"displayName": name}, token, 422)
	}
	f.call("PUT", "/v2/auth/profile", map[string]string{}, token, 422)
	f.call("PUT", "/v2/auth/profile", map[string]string{"displayName": "No session"}, "", 401)
	f.call("PUT", path, map[string]interface{}{"displayName": "Nama Baru", "isActive": false}, f.token, 200)
	user = mustUser(t, f.pool, id)
	if user.DisplayName == nil || *user.DisplayName != "Nama Baru" || user.IsActive {
		t.Fatal("administrator edit did not persist")
	}
	page := objectData(t, f.call("GET", "/v2/admin-users?search=Nama+Baru&role_code=konselor&is_active=false", nil, f.token, 200))
	var rows []database.Object
	if err := json.Unmarshal(page["data"], &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID("id") != id {
		t.Fatal("combined filters did not select matching account")
	}
	page = objectData(t, f.call("GET", "/v2/admin-users?search=Nama+Baru&is_active=true", nil, f.token, 200))
	if err := json.Unmarshal(page["data"], &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatal("active filter returned inactive account")
	}
	f.call("GET", "/v2/admin-users?is_active=invalid", nil, f.token, 422)
	unassigned := f.admin("")
	f.call("PUT", "/v2/auth/profile", map[string]string{"displayName": "Tanpa Role"}, f.tokenFor(unassigned), 200)
	page = objectData(t, f.call("GET", "/v2/admin-users?search=Tanpa+Role&role_code=unassigned", nil, f.token, 200))
	if err := json.Unmarshal(page["data"], &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID("id") != unassigned {
		t.Fatal("unassigned role filter failed")
	}
}
