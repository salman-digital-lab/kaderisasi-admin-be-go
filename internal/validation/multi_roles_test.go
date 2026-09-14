package validation

import (
	"encoding/json"
	"testing"
)

func TestAdminRoleCodesValidation(t *testing.T) {
	for _, raw := range []string{`[]`, `["konselor","activity_manager"]`} {
		out, issues := Validate("editAdminUser", Object{"role_codes": json.RawMessage(raw)})
		if len(issues) != 0 || !out.Has("role_codes") {
			t.Fatalf("valid roles rejected: %s %v", raw, issues)
		}
	}
	for _, raw := range []string{`null`, `"konselor"`, `[null]`, `[1]`, `["unknown"]`} {
		_, issues := Validate("editAdminUser", Object{"role_codes": json.RawMessage(raw)})
		if len(issues) == 0 {
			t.Fatalf("invalid roles accepted: %s", raw)
		}
	}
	_, issues := Validate("editAdminUser", Object{"role_code": json.RawMessage(`null`), "role_codes": json.RawMessage(`[]`)})
	if len(issues) == 0 {
		t.Fatal("conflicting role fields accepted")
	}
}
