package auth

import (
	"slices"
	"testing"
)

func TestMergedRoles(t *testing.T) {
	a := ForRoles([]string{"activity_manager", "konselor", "activity_manager", "retired"}, true)
	if len(a.Roles) != 2 || !a.Allows("activities.manage") || !a.Allows("counseling.manage") || a.Allows("admin_users.manage") {
		t.Fatalf("incorrect merged authorization: %+v", a)
	}
	seen := map[string]bool{}
	for _, permission := range a.Permissions {
		if seen[permission] {
			t.Fatalf("duplicate permission %s", permission)
		}
		seen[permission] = true
	}
	if inactive := ForRoles([]string{"activity_manager", "super_admin"}, false); len(inactive.Permissions) != 0 || inactive.IsSuperAdmin {
		t.Fatal("inactive account grants access")
	}
	if !ForRoles([]string{"konselor", "super_admin"}, true).IsSuperAdmin {
		t.Fatal("secondary Super Admin role ignored")
	}
}

// The p3ri role only opens the P3RI pages, so it is left out of the checks
// that every other role shares (calendar and short links).
const p3riOnly = "p3ri"

func TestCalendarAuthority(t *testing.T) {
	for _, role := range Roles() {
		if role.Code == p3riOnly {
			continue
		}
		access := ForRoles([]string{role.Code}, true)
		if !access.Allows("calendar.read") || access.Allows("calendar.manage") != (role.Code == "admin" || role.Code == "super_admin") {
			t.Fatalf("incorrect calendar authority: %s", role.Code)
		}
	}
	if !ForRoles([]string{"konselor", "admin"}, true).Allows("calendar.manage") {
		t.Fatal("secondary editor role ignored")
	}
	if ForRoles([]string{"activity_manager", "club_manager"}, true).Allows("calendar.manage") {
		t.Fatal("reader role union grants mutation")
	}
	if ForRoles([]string{"super_admin"}, false).Allows("calendar.manage") {
		t.Fatal("inactive editor allowed")
	}
}

func TestEightRolesAndPublicationAuthority(t *testing.T) {
	if len(Roles()) != 8 {
		t.Fatal("exactly eight active roles required")
	}
	for _, role := range Roles() {
		permission := ForRole(&role.Code, true)
		privileged := role.Code == "admin" || role.Code == "super_admin"
		for _, action := range []string{"activities.publish", "activities.registration.manage"} {
			if permission.Allows(action) != privileged {
				t.Fatalf("%s has wrong %s authority", role.Code, action)
			}
		}
		if len(role.Capabilities) == 0 || len(role.Capabilities) > 3 || role.Limitation == "" {
			t.Fatalf("%s needs concise guidance", role.Code)
		}
	}
	panitia := "activity_manager"
	if !ForRole(&panitia, true).Allows("activities.manage") {
		t.Fatal("Panitia must edit activities")
	}
	for _, retired := range []string{"member_manager", "asmen", "kapro", "leaderboard", "course_manager", "operations_admin", "counselor", "access_reviewer"} {
		if RoleByCode(retired) != nil || len(ForRole(&retired, true).Permissions) > 0 {
			t.Fatalf("retired role %s still grants access", retired)
		}
	}
}

func TestAchievementManagerInheritsOnlyPanitiaAndLeaderboardAccess(t *testing.T) {
	panitia := RoleByCode("activity_manager")
	manager := RoleByCode("achievement_manager")
	if panitia == nil || manager == nil || !manager.IsRequestable {
		t.Fatal("Panitia and requestable Pengelola Prestasi must exist")
	}
	expected := append(slices.Clone(panitia.Permissions),
		"achievements.export", "achievements.read", "achievements.review", "leaderboards.read")
	actual := slices.Clone(manager.Permissions)
	slices.Sort(expected)
	slices.Sort(actual)
	if !slices.Equal(actual, expected) {
		t.Fatalf("Pengelola Prestasi access = %v, want %v", actual, expected)
	}
}

func TestCounselingRestrictedToSuperAdminAndKonselor(t *testing.T) {
	for _, role := range Roles() {
		permission := ForRole(&role.Code, true)
		allowed := role.Code == "super_admin" || role.Code == "konselor"
		for _, action := range []string{"counseling.read", "counseling.manage"} {
			if permission.Allows(action) != allowed {
				t.Fatalf("%s has wrong %s authority", role.Code, action)
			}
		}
	}
}

func TestShortLinksAvailableToEveryActiveRole(t *testing.T) {
	for _, role := range Roles() {
		if role.Code == p3riOnly {
			continue
		}
		for _, permission := range []string{"short_links.read", "short_links.manage"} {
			if !ForRole(&role.Code, true).Allows(permission) {
				t.Fatalf("%s missing %s", role.Code, permission)
			}
			if ForRole(&role.Code, false).Allows(permission) {
				t.Fatalf("inactive %s grants %s", role.Code, permission)
			}
			if ForRole(nil, true).Allows(permission) {
				t.Fatalf("unassigned role grants %s", permission)
			}
		}
	}
}

func TestPerintisManagerIsAssignedOnly(t *testing.T) {
	role := RoleByCode("perintis_manager")
	if role == nil || role.IsRequestable {
		t.Fatal("Pengelola Perintis must exist and be assigned by Super Admin only")
	}
	access := ForRoles([]string{"perintis_manager"}, true)
	for _, permission := range []string{"perintis.participants.read", "perintis.participants.manage", "perintis.content.read", "perintis.followups.manage", "perintis.recap.read", "perintis.recap.export"} {
		if !access.Allows(permission) || !ForRoles([]string{"super_admin"}, true).Allows(permission) {
			t.Fatalf("missing %s", permission)
		}
	}
	if access.Allows("members.manage") || ForRoles([]string{"admin"}, true).Allows("perintis.content.read") {
		t.Fatal("perintis access leaks across roles")
	}
	// Searching every kaderisasi member to add participants is Super Admin only.
	if access.Allows("perintis.participants.add") || !ForRoles([]string{"super_admin"}, true).Allows("perintis.participants.add") {
		t.Fatal("perintis.participants.add must belong to Super Admin only")
	}
}

func TestP3riIsAssignedOnlyAndP3riOnly(t *testing.T) {
	role := RoleByCode("p3ri")
	if role == nil || role.IsRequestable {
		t.Fatal("Panitia P3RI must exist and be assigned by Super Admin only")
	}
	p3ri := []string{"p3ri.registrations.read", "p3ri.registrations.manage", "p3ri.registrations.export"}
	access := ForRoles([]string{"p3ri"}, true)
	actual := slices.Clone(access.Permissions)
	slices.Sort(actual)
	expected := slices.Clone(p3ri)
	slices.Sort(expected)
	if !slices.Equal(actual, expected) {
		t.Fatalf("Panitia P3RI access = %v, want only %v", actual, expected)
	}
	for _, permission := range p3ri {
		if !ForRoles([]string{"super_admin"}, true).Allows(permission) {
			t.Fatalf("Super Admin missing %s", permission)
		}
		for _, other := range Roles() {
			if other.Code != "p3ri" && other.Code != "super_admin" && ForRoles([]string{other.Code}, true).Allows(permission) {
				t.Fatalf("%s leaks %s", other.Code, permission)
			}
		}
	}
	if ForRoles([]string{"p3ri"}, false).Allows("p3ri.registrations.read") {
		t.Fatal("inactive Panitia P3RI allowed")
	}
}
