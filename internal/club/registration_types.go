package club

import (
	"encoding/json"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/member"
	"time"
)

type RegistrationInput struct {
	MemberID       json.Number                `json:"member_id"`
	Status         string                     `json:"status"`
	AdditionalData map[string]json.RawMessage `json:"additional_data"`
}
type RegistrationChange struct {
	ID json.Number `json:"id"`
	RegistrationInput
}
type RegistrationBatch struct {
	Registrations []RegistrationChange `json:"registrations"`
}
type RegistrationResponse struct {
	dbgen.ClubRegistration
	AdditionalData json.RawMessage           `json:"additional_data"`
	CreatedAt      *string                   `json:"created_at"`
	UpdatedAt      *string                   `json:"updated_at"`
	Member         *member.UserWithProfile   `json:"member"`
	Club           domain.Optional[Response] `json:"club,omitzero"`
	Roles          *[]RoleResponse           `json:"roles,omitempty"`
}
type RegistrationPage struct {
	Meta database.Pagination    `json:"meta"`
	Data []RegistrationResponse `json:"data"`
}
type RegistrationFilters struct {
	ClubID         string
	Status, Search *string
	Members        bool
	Ascending      bool
	Page, Size     float64
}
type RegistrationBatchResult struct {
	Data    []RegistrationResponse
	Missing []json.Number
}
type RoleInput struct {
	RegistrationID json.Number  `json:"club_registration_id"`
	Name           *string      `json:"role_name"`
	StartDate      *pgtype.Date `json:"start_date"`
	EndDate        *pgtype.Date `json:"end_date"`
	IsPrimary      *bool        `json:"is_primary"`
	SortOrder      *json.Number `json:"sort_order"`
}
type RoleResponse struct {
	dbgen.ClubMemberRole
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}
type RoleDetail struct {
	RoleResponse
	Registration RegistrationResponse `json:"registration"`
}

func roleView(row dbgen.ClubMemberRole) RoleResponse {
	return RoleResponse{ClubMemberRole: row, CreatedAt: domain.ModelTimestamp(row.CreatedAt, time.Local), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, time.Local)}
}
func registrationView(row dbgen.ClubRegistration, userJSON, profileJSON, clubJSON, rolesJSON []byte, profile, club, roles bool) (RegistrationResponse, error) {
	result := RegistrationResponse{ClubRegistration: row, AdditionalData: row.AdditionalData, CreatedAt: domain.ModelTimestamp(row.CreatedAt, time.Local), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, time.Local)}
	var err error
	result.Member, err = member.UserFromRelation(userJSON, profileJSON, profile)
	if err != nil {
		return result, err
	}
	if club {
		value, err := FromRelation(clubJSON)
		if err != nil {
			return result, err
		}
		result.Club = domain.Optional[Response]{Present: true, Value: value}
	}
	if roles {
		var rows []dbgen.ClubMemberRole
		if err = json.Unmarshal(rolesJSON, &rows); err != nil {
			return result, err
		}
		values := make([]RoleResponse, 0, len(rows))
		for _, role := range rows {
			values = append(values, roleView(role))
		}
		result.Roles = &values
	}
	return result, nil
}
