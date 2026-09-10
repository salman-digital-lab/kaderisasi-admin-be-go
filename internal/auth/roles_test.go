package auth

import "testing"

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
	for _, retired := range []string{"asmen", "kapro", "leaderboard", "course_manager", "operations_admin", "counselor", "access_reviewer"} {
		if RoleByCode(retired) != nil || len(ForRole(&retired, true).Permissions) > 0 {
			t.Fatalf("retired role %s still grants access", retired)
		}
	}
}
