package form

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

func (s Service) row(ctx context.Context, id string) (dbgen.CustomForm, error) {
	row, err := dbgen.New(s.Pool).FormByIdentifier(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, domain.Fail(404, "CUSTOM_FORM_NOT_FOUND")
	}
	return row, database.LegacyQueryError(err, `select * from "custom_forms" where "id" = $1 limit $2`)
}
func (s Service) requireClosed(ctx context.Context, row dbgen.CustomForm) error {
	id := currentClub(row)
	// This source controller uses a truth test, unlike the attachment service's
	// null check. A legacy zero attachment does not trigger its open-club guard.
	if id == nil || *id == "0" {
		return nil
	}
	club, err := dbgen.New(s.Pool).ClubByIdentifier(ctx, *id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if club.IsRegistrationOpen != nil && *club.IsRegistrationOpen {
		return domain.Fail(400, "CLOSE_REGISTRATION_BEFORE_FORM_CHANGE")
	}
	return nil
}
func (s Service) Delete(ctx context.Context, id string) error {
	row, err := s.row(ctx, id)
	if err != nil {
		return err
	}
	if err = s.requireClosed(ctx, row); err != nil {
		return err
	}
	return dbgen.New(s.Pool).DeleteForm(ctx, row.ID)
}
func (s Service) Toggle(ctx context.Context, id string) (Response, error) {
	row, err := s.row(ctx, id)
	if err != nil {
		return Response{}, err
	}
	active := row.IsActive != nil && *row.IsActive
	if active {
		if err = s.requireClosed(ctx, row); err != nil {
			return Response{}, err
		}
	}
	updated, err := dbgen.New(s.Pool).ToggleFormActive(ctx, dbgen.ToggleFormActiveParams{ID: row.ID, IsActive: !active})
	return View(updated, s.Location), err
}
func (s Service) AttachActivity(ctx context.Context, id string, input ActivityAttachment) (ActivityAttached, error) {
	if !database.JSONTruthy(input.ActivityID) {
		return ActivityAttached{}, domain.Fail(400, "ACTIVITY_ID_REQUIRED")
	}
	row, err := s.row(ctx, id)
	if err != nil {
		return ActivityAttached{}, err
	}
	if row.FeatureID != nil && *row.FeatureID != 0 {
		return ActivityAttached{}, domain.Fail(400, "FORM_ALREADY_ATTACHED")
	}
	next, err := dbgen.New(s.Pool).AttachFormToActivity(ctx, dbgen.AttachFormToActivityParams{ID: row.ID, Identifier: database.JSONParameter(input.ActivityID)})
	statement := `update "custom_forms" set "feature_id" = $1, "updated_at" = $2, "feature_type" = $3 where "id" = $4`
	if row.FeatureType != nil && *row.FeatureType == "activity_registration" {
		statement = `update "custom_forms" set "feature_id" = $1, "updated_at" = $2 where "id" = $3`
	}
	return ActivityAttached{Response: View(next, s.Location), FeatureID: input.ActivityID}, database.LegacyQueryError(err, statement)
}
func (s Service) DetachActivity(ctx context.Context, id string) (Response, error) {
	row, err := s.row(ctx, id)
	if err != nil {
		return Response{}, err
	}
	row, err = dbgen.New(s.Pool).DetachFormFromActivity(ctx, row.ID)
	return View(row, s.Location), err
}
