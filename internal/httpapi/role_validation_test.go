package httpapi

import (
	"encoding/json"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/validation"
	"slices"
	"testing"
)

func TestRoleValidationMatchesAuthorizationCatalog(t *testing.T) {
	expected := make([]string, 0, len(auth.Roles()))
	for _, role := range auth.Roles() {
		expected = append(expected, role.Code)
	}
	for _, name := range []string{"registerValidator", "editAdminUser", "createAccessRequestValidator"} {
		_, issues := validation.ValidateField(name, "role_code", json.RawMessage(`"unknown-role"`))
		if len(issues) != 1 || issues[0].Rule != "enum" {
			t.Fatalf("%s did not reject an unknown role", name)
		}
		raw, _ := json.Marshal(issues[0].Meta["choices"])
		var actual []string
		if err := json.Unmarshal(raw, &actual); err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(actual, expected) {
			t.Fatalf("%s role choices %v differ from the authorization catalog %v", name, actual, expected)
		}
	}
}
