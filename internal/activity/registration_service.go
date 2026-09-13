package activity

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/member"
	"slices"
)

func intValue(value *int32) int32 {
	if value == nil {
		return 0
	}
	return *value
}
func (s Service) Register(ctx context.Context, identifier string, data RegistrationInput) (CreatedRegistration, error) {
	q := dbgen.New(s.Pool)
	profile, err := q.ProfileForRegistration(ctx, data.ProfileID.String())
	if err != nil {
		return CreatedRegistration{}, database.LegacyQueryError(err, `select * from "profiles" where "id" = $1 limit $2`)
	}
	activity, err := q.RegistrationActivityByIdentifier(ctx, identifier)
	if err != nil {
		return CreatedRegistration{}, database.LegacyQueryError(err, `select * from "activities" where "id" = $1 limit $2`)
	}
	id := activity.ID
	count, err := q.CountMemberRegistrations(ctx, dbgen.CountMemberRegistrationsParams{UserID: profile.UserID, ActivityID: id})
	if err != nil {
		return CreatedRegistration{}, err
	}
	if count > 0 {
		return CreatedRegistration{}, domain.Fail(409, "ALREADY_REGISTERED")
	}
	if intValue(profile.Level) < intValue(activity.MinimumLevel) {
		return CreatedRegistration{}, domain.Fail(403, "UNMATCHED_LEVEL")
	}
	raw, err := json.Marshal(data.QuestionnaireAnswer)
	if err != nil {
		return CreatedRegistration{}, err
	}
	row, err := q.CreateActivityRegistration(ctx, dbgen.CreateActivityRegistrationParams{UserID: profile.UserID, ActivityID: &id, QuestionnaireAnswer: raw})
	return CreatedRegistrationView(row), err
}
func (s Service) RegistrationStatistics(ctx context.Context, id string) (RegistrationStatistics, error) {
	q := dbgen.New(s.Pool)
	rows, err := q.RegistrationStatusCounts(ctx, id)
	if err != nil {
		return RegistrationStatistics{}, database.LegacyQueryError(err, `select "status", count(*) as "count" from "activity_registrations" where "activity_id" = $1 group by "status"`)
	}
	total, err := q.RegistrationTotal(ctx, id)
	if err != nil {
		return RegistrationStatistics{}, err
	}
	result := RegistrationStatistics{Total: total, ByStatus: map[string]int64{}}
	for _, row := range rows {
		label := "null"
		if row.Status != nil {
			label = *row.Status
		}
		result.ByStatus[label] = row.Count
	}
	return result, nil
}
func (s Service) DeleteRegistration(ctx context.Context, identifier string) (bool, error) {
	q := dbgen.New(s.Pool)
	row, err := q.ActivityRegistrationByID(ctx, identifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, database.LegacyQueryError(err, `select * from "activity_registrations" where "id" = $1 limit $2`)
	}
	id := row.ID
	if history, err := q.RegistrationHasScoringPublications(ctx, id); err != nil {
		return false, err
	} else if history {
		return false, domain.Fail(409, "REGISTRATION_HAS_SCORING_HISTORY")
	}
	exists, err := q.RegistrationHasCertificate(ctx, id)
	if err != nil {
		return false, err
	}
	if exists {
		return false, domain.Fail(409, "CERTIFICATE_REGISTRATION_HAS_ISSUED_CERTIFICATE")
	}
	_, err = q.DeleteActivityRegistration(ctx, id)
	var constraint *pgconn.PgError
	if errors.As(err, &constraint) && constraint.Code == "23503" && constraint.TableName == "activity_scoring_publications" {
		return false, domain.Fail(409, "REGISTRATION_HAS_SCORING_HISTORY")
	}
	return true, err
}

func registrationNumbers(ids []json.Number) []string {
	result := make([]string, len(ids))
	for i, id := range ids {
		result[i] = id.String()
	}
	return result
}
func specialGraduation(activity dbgen.Activity, status string) bool {
	kind := intValue(activity.ActivityType)
	return status == "LULUS KEGIATAN" && kind >= 2 && kind <= 5
}
func upgradeRegistrationProfiles(ctx context.Context, q *dbgen.Queries, activity dbgen.Activity, users []int32, status string) error {
	if !specialGraduation(activity, status) {
		return nil
	}
	level, ok := map[int32]int32{2: 3, 3: 6, 5: 10}[intValue(activity.ActivityType)]
	if !ok {
		return errors.New("Empty .update() call detected! Update data does not contain any values to update. This will result in a faulty query. Table: profiles. Columns: level.")
	}
	if err := q.SetRegistrationProfileLevel(ctx, dbgen.SetRegistrationProfileLevelParams{Level: &level, Column2: users}); err != nil {
		return err
	}
	if activity.Badge == nil || *activity.Badge == "" {
		return nil
	}
	profiles, err := q.RegistrationProfilesByUsers(ctx, users)
	if err != nil {
		return err
	}
	for _, profile := range profiles {
		badges := member.ProfileView(profile).Badges
		if !slices.Contains(badges, *activity.Badge) {
			raw, err := json.Marshal(append(badges, *activity.Badge))
			if err != nil {
				return err
			}
			if err = q.SetRegistrationProfileBadges(ctx, dbgen.SetRegistrationProfileBadgesParams{ID: profile.ID, Badges: raw}); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s Service) ChangeRegistrationStatuses(ctx context.Context, data RegistrationStatusInput) (int64, error) {
	if len(data.IDs) == 0 {
		return 0, errors.New(`"findOrFail" expects a value. Received undefined`)
	}
	activity, err := dbgen.New(s.Pool).ActivityForRegistration(ctx, data.IDs[0].String())
	if err != nil {
		return 0, database.LegacyQueryError(err, `select * from "activity_registrations" where "id" = $1 limit $2`)
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	ids := registrationNumbers(data.IDs)
	if specialGraduation(activity, data.Status) {
		rows, err := q.UsersForRegistrationIDs(ctx, ids)
		if err != nil {
			return 0, err
		}
		users := []int32{}
		for _, id := range rows {
			if id != nil {
				users = append(users, *id)
			}
		}
		if err = upgradeRegistrationProfiles(ctx, q, activity, users, data.Status); err != nil {
			return 0, err
		}
	}
	count, err := q.ChangeRegistrationStatusByIDs(ctx, dbgen.ChangeRegistrationStatusByIDsParams{Status: data.Status, Ids: ids})
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}
func (s Service) ChangeRegistrationStatusesByEmail(ctx context.Context, identifier string, data RegistrationEmailStatusInput) (int64, error) {
	activity, err := dbgen.New(s.Pool).RegistrationActivityByIdentifier(ctx, identifier)
	if err != nil {
		return 0, database.LegacyQueryError(err, `select * from "activities" where "id" = $1 limit $2`)
	}
	id := activity.ID
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	users, err := q.RegistrationPublicUsersByEmail(ctx, data.Emails)
	if err != nil {
		return 0, err
	}
	if len(users) == 0 {
		return 0, domain.Fail(404, "NO_USERS_FOUND")
	}
	registrations, err := q.RegistrationsByActivityUsers(ctx, dbgen.RegistrationsByActivityUsersParams{ActivityID: &id, Column2: users})
	if err != nil {
		return 0, err
	}
	if len(registrations) == 0 {
		return 0, domain.Fail(404, "NO_REGISTRATIONS_FOUND")
	}
	if err = upgradeRegistrationProfiles(ctx, q, activity, users, data.Status); err != nil {
		return 0, err
	}
	count, err := q.ChangeRegistrationStatusByUsers(ctx, dbgen.ChangeRegistrationStatusByUsersParams{Status: &data.Status, ActivityID: &id, Column3: users})
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return count, nil
}
func (s Service) ChangeRegistrationStatusesBulk(ctx context.Context, id string, data RegistrationBulkStatusInput) (int64, error) {
	// The source endpoint applies an invalid name column filter. Keep its actual
	// PostgreSQL failure; such a query intentionally cannot be generated by sqlc.
	if data.Name != nil && *data.Name != "" {
		query := "UPDATE activity_registrations SET status=$1 WHERE activity_id=$2 AND name=$3"
		diagnostic := `update "activity_registrations" set "status" = $1 where "activity_id" = $2 and "name" = $3`
		args := []any{data.NewStatus, id, *data.Name}
		if data.CurrentStatus != nil && *data.CurrentStatus != "" {
			query += " AND status=$4"
			diagnostic += ` and "status" = $4`
			args = append(args, *data.CurrentStatus)
		}
		tag, err := s.Pool.Exec(ctx, query, args...)
		return tag.RowsAffected(), database.LegacyQueryError(err, diagnostic)
	}
	count, err := dbgen.New(s.Pool).ChangeRegistrationStatusBulk(ctx, dbgen.ChangeRegistrationStatusBulkParams{ActivityID: id, NewStatus: data.NewStatus, CurrentStatus: data.CurrentStatus})
	statement := `update "activity_registrations" set "status" = $1 where "activity_id" = $2`
	if data.CurrentStatus != nil {
		statement += ` and "status" = $3`
	}
	return count, database.LegacyQueryError(err, statement)
}
