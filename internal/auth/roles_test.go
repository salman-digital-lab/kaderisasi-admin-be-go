package auth

import (
	"slices"
	"testing"
)

func TestSixRolesAndPublicationAuthority(t *testing.T) {
	if len(Roles()) != 6 {
		t.Fatal("exactly six active roles required")
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
