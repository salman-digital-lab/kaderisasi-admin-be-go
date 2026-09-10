package club

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"strconv"
)

func (s Service) RequireRegistrationClub(ctx context.Context, id string) error {
	_, err := s.registrationClub(ctx, id)
	return err
}
func roleDetails(ctx context.Context, q *dbgen.Queries, row dbgen.ClubMemberRole) (RoleDetail, error) {
	registration, err := registrationDetails(ctx, q, row.ClubRegistrationID, true, false, false)
	return RoleDetail{RoleResponse: roleView(row), Registration: registration}, err
}
func (s Service) Roles(ctx context.Context, id string) ([]RoleDetail, error) {
	club, err := s.registrationClub(ctx, id)
	if err != nil {
		return nil, err
	}
	q := dbgen.New(s.Pool)
	roles, err := q.ClubRoleList(ctx, club.ID)
	if err != nil {
		return nil, err
	}
	result := make([]RoleDetail, 0, len(roles))
	for _, row := range roles {
		registration, err := registrationView(row.ClubRegistration, row.Member, row.Profile, nil, nil, true, false, false)
		if err != nil {
			return nil, err
		}
		result = append(result, RoleDetail{RoleResponse: roleView(row.ClubMemberRole), Registration: registration})
	}
	return result, nil
}
func (s Service) RoleSuggestions(ctx context.Context, id string) ([]string, error) {
	club, err := s.registrationClub(ctx, id)
	if err != nil {
		return nil, err
	}
	return dbgen.New(s.Pool).ClubRoleSuggestions(ctx, club.ID)
}
func (s Service) CreateRole(ctx context.Context, id string, input RoleInput) (RoleDetail, error) {
	club, err := s.registrationClub(ctx, id)
	if err != nil {
		return RoleDetail{}, err
	}
	q := dbgen.New(s.Pool)
	registration, err := q.ApprovedClubRoleRegistration(ctx, dbgen.ApprovedClubRoleRegistrationParams{ClubID: club.ID, Identifier: input.RegistrationID.String()})
	if errors.Is(err, pgx.ErrNoRows) {
		return RoleDetail{}, domain.Fail(400, "APPROVED_CLUB_MEMBER_REQUIRED")
	}
	if err != nil {
		return RoleDetail{}, database.LegacyQueryError(err, `select * from "club_registrations" where "id" = $1 and "club_id" = $2 and "status" = $3 limit $4`)
	}
	params := dbgen.CreateClubRoleParams{RegistrationID: registration.ID, RoleName: *input.Name, SortOrder: "0"}
	if input.StartDate != nil {
		params.StartDate = *input.StartDate
	}
	if input.EndDate != nil {
		params.EndDate = *input.EndDate
	}
	if input.SortOrder != nil {
		params.SortOrder = input.SortOrder.String()
	}
	if input.IsPrimary != nil {
		params.IsPrimary = *input.IsPrimary
	}
	if params.IsPrimary {
		if err = q.ClearPrimaryClubRoles(ctx, dbgen.ClearPrimaryClubRolesParams{RegistrationID: registration.ID}); err != nil {
			return RoleDetail{}, err
		}
	}
	row, err := q.CreateClubRole(ctx, params)
	if err != nil {
		return RoleDetail{}, database.LegacyQueryError(err, `insert into "club_member_roles" ("club_registration_id", "created_at", "end_date", "is_primary", "role_name", "sort_order", "start_date", "updated_at") values ($1, $2, $3, $4, $5, $6, $7, $8) returning "id"`)
	}
	return roleDetails(ctx, q, row)
}
func roleRow(ctx context.Context, q *dbgen.Queries, id string) (dbgen.ClubMemberRole, error) {
	row, err := q.ClubRoleByIdentifier(ctx, id)
	return row, database.LegacyQueryError(err, `select * from "club_member_roles" where "id" = $1 limit $2`)
}
func (s Service) UpdateRole(ctx context.Context, id string, input RoleInput) (RoleDetail, error) {
	q := dbgen.New(s.Pool)
	row, err := roleRow(ctx, q, id)
	if err != nil {
		return RoleDetail{}, err
	}
	params := dbgen.UpdateClubRoleParams{ID: row.ID, RoleName: row.RoleName, StartDate: row.StartDate, EndDate: row.EndDate, IsPrimary: row.IsPrimary, SortOrder: strconv.FormatInt(int64(row.SortOrder), 10)}
	if input.Name != nil {
		params.RoleName = *input.Name
	}
	if input.StartDate != nil {
		params.StartDate = *input.StartDate
	}
	if input.EndDate != nil {
		params.EndDate = *input.EndDate
	}
	if input.SortOrder != nil {
		params.SortOrder = input.SortOrder.String()
	}
	if input.IsPrimary != nil {
		params.IsPrimary = *input.IsPrimary
		if params.IsPrimary {
			if err = q.ClearPrimaryClubRoles(ctx, dbgen.ClearPrimaryClubRolesParams{RegistrationID: row.ClubRegistrationID, ExceptID: &row.ID}); err != nil {
				return RoleDetail{}, err
			}
		}
	}
	updated, err := q.UpdateClubRole(ctx, params)
	if err != nil {
		return RoleDetail{}, database.LegacyQueryError(err, roleUpdateStatement(row, params))
	}
	return roleDetails(ctx, q, updated)
}
func (s Service) DeleteRole(ctx context.Context, id string) error {
	q := dbgen.New(s.Pool)
	row, err := roleRow(ctx, q, id)
	if err != nil {
		return err
	}
	return q.DeleteClubRole(ctx, row.ID)
}

func roleUpdateStatement(row dbgen.ClubMemberRole, params dbgen.UpdateClubRoleParams) string {
	columns := []string{}
	if row.RoleName != params.RoleName {
		columns = append(columns, "role_name")
	}
	if row.StartDate != params.StartDate {
		columns = append(columns, "start_date")
	}
	if row.EndDate != params.EndDate {
		columns = append(columns, "end_date")
	}
	if row.IsPrimary != params.IsPrimary {
		columns = append(columns, "is_primary")
	}
	if strconv.FormatInt(int64(row.SortOrder), 10) != params.SortOrder {
		columns = append(columns, "sort_order")
	}
	columns = append(columns, "updated_at")
	result := `update "club_member_roles" set `
	for i, column := range columns {
		if i > 0 {
			result += ", "
		}
		result += `"` + column + `" = $` + strconv.Itoa(i+1)
	}
	return result + ` where "id" = $` + strconv.Itoa(len(columns)+1)
}
