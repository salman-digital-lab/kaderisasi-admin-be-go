package validation

import (
	"encoding/json"
	"testing"
)

func TestActiveRoleInputs(t *testing.T) {
	for _, validator := range []string{"registerValidator", "editAdminUser", "createAccessRequestValidator"} {
		t.Run(validator, func(t *testing.T) {
			raw, issues := ValidateField(validator, "role_code", json.RawMessage(`"achievement_manager"`))
			if len(issues) != 0 || string(raw) != `"achievement_manager"` {
				t.Fatalf("new role rejected: %s %v", raw, issues)
			}
			for _, retired := range []string{"member_manager", "kapro", "asmen", "leaderboard", "operations_admin", "achievement_reviewer"} {
				_, issues = ValidateField(validator, "role_code", marshal(retired))
				if len(issues) != 1 || issues[0].Rule != "enum" {
					t.Fatalf("retired role %s accepted: %v", retired, issues)
				}
			}
		})
	}
}
