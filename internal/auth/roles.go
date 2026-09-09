package auth

import (
	_ "embed"
	"encoding/json"
	"slices"
)

//go:embed roles.json
var rolesJSON []byte

//go:embed permissions.json
var permissionsJSON []byte

func Permissions() []string {
	var permissions []string
	if err := json.Unmarshal(permissionsJSON, &permissions); err != nil {
		panic(err)
	}
	return permissions
}

type Role struct {
	Code          string   `json:"code"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	IsRequestable bool     `json:"is_requestable"`
	Permissions   []string `json:"permissions"`
}

type AssignedRole struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
type Authorization struct {
	Role         *AssignedRole `json:"role"`
	Permissions  []string      `json:"permissions"`
	IsSuperAdmin bool          `json:"is_super_admin"`
}

func Roles() []Role {
	var roles []Role
	if err := json.Unmarshal(rolesJSON, &roles); err != nil {
		panic(err)
	}
	return roles
}
func RoleByCode(code string) *Role {
	for _, role := range Roles() {
		if role.Code == code {
			return &role
		}
	}
	return nil
}
func ForRole(code *string, active bool) Authorization {
	result := Authorization{Permissions: []string{}}
	if !active || code == nil {
		return result
	}
	if role := RoleByCode(*code); role != nil {
		result.Role = &AssignedRole{Code: role.Code, Name: role.Name}
		result.Permissions = role.Permissions
		result.IsSuperAdmin = role.Code == "super_admin"
	}
	return result
}
func (a Authorization) Allows(permission string) bool {
	return slices.Contains(a.Permissions, permission)
}
