package club

import (
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"strconv"
	"strings"
)

func (s Service) registrationClub(ctx context.Context, id string) (dbgen.Club, error) {
	row, err := dbgen.New(s.Pool).ClubByIdentifier(ctx, id)
	return row, database.LegacyQueryError(err, `select * from "clubs" where "id" = $1 limit $2`)
}
func registrationRow(ctx context.Context, q *dbgen.Queries, id string) (dbgen.ClubRegistration, error) {
	row, err := q.ClubRegistrationByIdentifier(ctx, id)
	return row, database.LegacyQueryError(err, `select * from "club_registrations" where "id" = $1 limit $2`)
}
func registrationDetails(ctx context.Context, q *dbgen.Queries, id int32, profile, club, roles bool) (RegistrationResponse, error) {
	row, err := q.ClubRegistrationDetails(ctx, dbgen.ClubRegistrationDetailsParams{ID: id})
	if err != nil {
		return RegistrationResponse{}, err
	}
	return registrationView(row.ClubRegistration, row.Member, row.Profile, row.Club, row.Roles, profile, club, roles)
}
func (s Service) Registrations(ctx context.Context, filters RegistrationFilters) (RegistrationPage, error) {
	result := RegistrationPage{Data: []RegistrationResponse{}}
	club, err := s.registrationClub(ctx, filters.ClubID)
	if err != nil {
		return result, err
	}
	if filters.Members {
		status := "APPROVED"
		filters.Status = &status
	} else {
		filters.Search = nil
	}
	q := dbgen.New(s.Pool)
	total, err := q.CountClubRegistrations(ctx, dbgen.CountClubRegistrationsParams{ClubID: club.ID, Status: filters.Status, Search: filters.Search})
	result.Meta = database.Meta(total, filters.Page, filters.Size)
	if err != nil || total == 0 {
		return result, err
	}
	size, offset, err := database.SQLPage(filters.Page, filters.Size)
	if err != nil {
		return result, registrationPageError(err, filters)
	}
	rows, err := q.ListClubRegistrations(ctx, dbgen.ListClubRegistrationsParams{ClubID: club.ID, Status: filters.Status, Search: filters.Search, Ascending: filters.Ascending, MemberOrder: filters.Members, PageSize: size, PageOffset: offset})
	if err != nil {
		return result, registrationPageError(err, filters)
	}
	for _, row := range rows {
		view, err := registrationView(row.ClubRegistration, row.Member, row.Profile, row.Club, row.Roles, true, false, true)
		if err != nil {
			return result, err
		}
		result.Data = append(result.Data, view)
	}
	return result, nil
}
func (s Service) Registration(ctx context.Context, id string) (RegistrationResponse, error) {
	q := dbgen.New(s.Pool)
	row, err := registrationRow(ctx, q, id)
	if err != nil {
		return RegistrationResponse{}, err
	}
	return registrationDetails(ctx, q, row.ID, true, true, true)
}
func (s Service) CreateRegistration(ctx context.Context, id string, input RegistrationInput) (RegistrationResponse, error) {
	club, err := s.registrationClub(ctx, id)
	if err != nil {
		return RegistrationResponse{}, err
	}
	q := dbgen.New(s.Pool)
	user, err := q.ClubRegistrationUser(ctx, input.MemberID.String())
	if err != nil {
		return RegistrationResponse{}, database.LegacyQueryError(err, `select * from "public_users" where "id" = $1 limit $2`)
	}
	exists, err := q.ClubRegistrationDuplicate(ctx, dbgen.ClubRegistrationDuplicateParams{ClubID: club.ID, MemberID: user.ID})
	if err != nil {
		return RegistrationResponse{}, err
	}
	if exists {
		return RegistrationResponse{}, domain.Fail(409, "MEMBER_ALREADY_REGISTERED")
	}
	data := []byte(`{}`)
	if input.AdditionalData != nil {
		data, err = json.Marshal(input.AdditionalData)
		if err != nil {
			return RegistrationResponse{}, err
		}
	}
	row, err := q.CreateClubRegistration(ctx, dbgen.CreateClubRegistrationParams{ClubID: club.ID, MemberID: user.ID, AdditionalData: data})
	if err != nil {
		return RegistrationResponse{}, database.LegacyQueryError(err, `insert into "club_registrations" ("additional_data", "club_id", "created_at", "member_id", "status", "updated_at") values ($1, $2, $3, $4, $5, $6) returning "id"`)
	}
	return registrationDetails(ctx, q, row.ID, false, true, false)
}
func updateRegistration(ctx context.Context, q *dbgen.Queries, row dbgen.ClubRegistration, input RegistrationInput) (dbgen.ClubRegistration, error) {
	data := row.AdditionalData
	if input.AdditionalData != nil {
		var err error
		data, err = json.Marshal(input.AdditionalData)
		if err != nil {
			return row, err
		}
	}
	return q.UpdateClubRegistration(ctx, dbgen.UpdateClubRegistrationParams{ID: row.ID, Status: input.Status, AdditionalData: data})
}
func (s Service) UpdateRegistration(ctx context.Context, id string, input RegistrationInput) (RegistrationResponse, error) {
	q := dbgen.New(s.Pool)
	row, err := registrationRow(ctx, q, id)
	if err != nil {
		return RegistrationResponse{}, err
	}
	row, err = updateRegistration(ctx, q, row, input)
	if err != nil {
		return RegistrationResponse{}, err
	}
	return registrationDetails(ctx, q, row.ID, false, true, false)
}
func (s Service) BulkRegistrations(ctx context.Context, input RegistrationBatch) (RegistrationBatchResult, error) {
	result := RegistrationBatchResult{Data: []RegistrationResponse{}, Missing: []json.Number{}}
	ids := []string{}
	seen := map[string]bool{}
	for _, change := range input.Registrations {
		id := change.ID.String()
		if seen[id] {
			return result, domain.Fail(400, "DUPLICATE_REGISTRATION_IDS")
		}
		ids = append(ids, id)
		seen[id] = true
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	rows, err := q.LockClubRegistrations(ctx, ids)
	if err != nil {
		params := []string{}
		for i := range ids {
			params = append(params, fmt.Sprintf("$%d", i+1))
		}
		return result, database.LegacyQueryError(err, `select * from "club_registrations" where "id" in (`+strings.Join(params, ", ")+`) for update`)
	}
	byID := map[string]dbgen.ClubRegistration{}
	for _, row := range rows {
		byID[strconv.FormatInt(int64(row.ID), 10)] = row
	}
	for _, change := range input.Registrations {
		if _, ok := byID[change.ID.String()]; !ok {
			result.Missing = append(result.Missing, change.ID)
		}
	}
	if len(result.Missing) > 0 {
		return result, tx.Commit(ctx)
	}
	// Relations are loaded before mutation in Adonis and retain request order.
	for _, change := range input.Registrations {
		row := byID[change.ID.String()]
		view, err := registrationDetails(ctx, q, row.ID, false, true, false)
		if err != nil {
			return result, err
		}
		updated, err := updateRegistration(ctx, q, row, change.RegistrationInput)
		if err != nil {
			return result, err
		}
		view.ClubRegistration = updated
		view.AdditionalData = updated.AdditionalData
		view.UpdatedAt = domain.ModelTimestamp(updated.UpdatedAt, s.location())
		result.Data = append(result.Data, view)
	}
	return result, tx.Commit(ctx)
}
func (s Service) DeleteRegistration(ctx context.Context, id string) error {
	q := dbgen.New(s.Pool)
	row, err := registrationRow(ctx, q, id)
	if err != nil {
		return err
	}
	return q.DeleteClubRegistration(ctx, row.ID)
}
func registrationPageError(err error, filters RegistrationFilters) error {
	// Keep the source's query diagnostic; generated parameters remain private.
	n := 1
	conditions := `"club_id" = $1`
	if filters.Status != nil {
		n++
		conditions += fmt.Sprintf(` and "status" = $%d`, n)
	}
	if filters.Search != nil {
		n++
		email := n
		n++
		conditions += fmt.Sprintf(` and exists (select * from "public_users" where ("email" ilike $%d or exists (select * from "profiles" where ("name" ilike $%d) and ("public_users"."id" = "profiles"."user_id"))) and ("public_users"."id" = "club_registrations"."member_id"))`, email, n)
	}
	order := "desc"
	if filters.Ascending {
		order = "asc"
	}
	statement := `select * from "club_registrations" where ` + conditions + ` order by "created_at" ` + order + ` nulls last, "id" ` + order
	n++
	statement += fmt.Sprintf(" limit $%d", n)
	if filters.Page != 1 {
		n++
		statement += fmt.Sprintf(" offset $%d", n)
	}
	return database.LegacyQueryError(err, statement)
}
