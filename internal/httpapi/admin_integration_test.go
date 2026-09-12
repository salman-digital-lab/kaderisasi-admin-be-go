//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"testing"
)

func mustUser(t *testing.T, pool *pgxpool.Pool, id int32) dbgen.AdminUser {
	t.Helper()
	user, err := dbgen.New(pool).FindAdminByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return user
}

func TestAchievementManagerRequestAndCurrentSessionPermissions(t *testing.T) {
	f := newHTTPFixture(t)
	id := f.admin("")
	token := f.tokenFor(id)
	targets := objectData(t, f.call("GET", "/v2/rbac/requestable-targets", nil, token, 200))
	var roles []auth.Role
	if err := json.Unmarshal(targets["roles"], &roles); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, role := range roles {
		if role.Code == "achievement_manager" {
			found = role.IsRequestable && role.Name == "Pengelola Prestasi"
		}
	}
	if !found {
		t.Fatal("Pengelola Prestasi missing from requestable targets")
	}
	f.call("GET", "/v2/leaderboards/lifetime", nil, token, 403)
	request := objectData(t, f.call("POST", "/v2/access-requests",
		map[string]string{"role_code": "achievement_manager", "reason": "Mengelola kegiatan dan memeriksa prestasi"}, token, 201))
	f.call("POST", fmt.Sprintf("/v2/tickets/review/%d/approve", request.ID("id")), nil, f.token, 200)
	if fresh := mustUser(t, f.pool, id); fresh.RoleCode == nil || *fresh.RoleCode != "achievement_manager" {
		t.Fatal("approved Pengelola Prestasi role not applied")
	}
	// The same JWT must use the current role, including after access is removed.
	f.call("GET", "/v2/leaderboards/lifetime", nil, token, 200)
	f.call("GET", "/v2/activities", nil, token, 200)
	userPath := fmt.Sprintf("/v2/admin-users/%d", id)
	f.call("PUT", userPath, map[string]string{"role_code": "konselor"}, f.token, 200)
	f.call("GET", "/v2/ruang-curhat", nil, token, 200)
	f.call("PUT", userPath, map[string]string{"role_code": "admin"}, f.token, 200)
	f.call("GET", "/v2/ruang-curhat", nil, token, 403)
	f.call("GET", "/v2/activities", nil, token, 200)
	f.call("PUT", userPath, map[string]string{"role_code": "achievement_manager"}, f.token, 200)
	f.call("GET", "/v2/leaderboards/lifetime", nil, token, 200)
}
func TestAdministratorAndTicketWorkflows(t *testing.T) {
	f := newHTTPFixture(t)
	uuid, err := auth.UUID()
	if err != nil {
		t.Fatal(err)
	}
	created := objectData(t, f.call("POST", "/v2/admin-users", map[string]interface{}{"displayName": "New fixture", "email": uuid + "@example.test", "password": "Fixture-password-2026!", "role_code": nil}, f.token, 201))
	id := created.ID("id")
	f.admins = append(f.admins, id)
	if created.Has("password") {
		t.Fatal("password leaked")
	}
	f.call("GET", "/v2/admin-users?page=1&per_page=2&search=fixture", nil, f.token, 200)
	f.call("GET", fmt.Sprintf("/v2/admin-users/%d", id), nil, f.token, 200)
	f.call("PUT", fmt.Sprintf("/v2/admin-users/%d", id), map[string]string{"role_code": "konselor"}, f.token, 200)
	f.call("PUT", fmt.Sprintf("/v2/admin-users/%d/password", id), map[string]string{"password": "Updated-password"}, f.token, 200)
	f.call("PUT", fmt.Sprintf("/v2/admin-users/%d", f.adminID), map[string]bool{"isActive": false}, f.token, 409)
	f.call("PUT", fmt.Sprintf("/v2/admin-users/%d", f.adminID), map[string]interface{}{"role_code": nil}, f.token, 409)
	user := mustUser(t, f.pool, id)
	// Obtain the real session through the public HTTP contract.
	login := objectData(t, f.call("POST", "/v2/auth/login", map[string]string{"email": user.Email, "password": "Updated-password"}, "", 200))
	token := login.String("access_token")
	f.call("GET", "/v2/admin-users", nil, token, 403)
	request := objectData(t, f.call("POST", "/v2/access-requests", map[string]string{"role_code": "club_manager", "reason": "Need fixture club access"}, token, 201))
	ticketID := request.ID("id")
	f.call("POST", "/v2/access-requests", map[string]string{"role_code": "club_manager", "reason": "Duplicate fixture access"}, token, 409)
	f.call("GET", "/v2/access-requests", nil, token, 200)
	f.call("GET", fmt.Sprintf("/v2/access-requests/%d", ticketID), nil, token, 200)
	f.call("GET", fmt.Sprintf("/v2/access-requests/%d", ticketID), nil, f.token, 403)
	f.call("GET", "/v2/tickets/review?status=open", nil, f.token, 200)
	f.call("GET", fmt.Sprintf("/v2/tickets/review/%d", ticketID), nil, f.token, 200)
	f.call("POST", fmt.Sprintf("/v2/tickets/review/%d/approve", ticketID), nil, f.token, 200)
	if fresh := mustUser(t, f.pool, id); fresh.RoleCode == nil || *fresh.RoleCode != "club_manager" {
		t.Fatal("approved role not applied")
	}
	f.call("POST", fmt.Sprintf("/v2/tickets/review/%d/approve", ticketID), nil, f.token, 409)
	second := objectData(t, f.call("POST", "/v2/access-requests", map[string]string{"role_code": "konselor", "reason": "Second fixture request"}, token, 201))
	f.call("POST", fmt.Sprintf("/v2/tickets/review/%d/reject", second.ID("id")), map[string]string{"rejection_reason": "Fixture request rejected"}, f.token, 200)
	third := objectData(t, f.call("POST", "/v2/access-requests", map[string]string{"role_code": "konselor", "reason": "Cancelled fixture request"}, token, 201))
	f.call("POST", fmt.Sprintf("/v2/access-requests/%d/cancel", third.ID("id")), nil, token, 200)
}
