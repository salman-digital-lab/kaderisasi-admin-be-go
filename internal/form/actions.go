package form

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

func lockedRow(ctx context.Context, q *dbgen.Queries, id string) (dbgen.CustomForm, error) {
	row, err := q.LockFormByIdentifier(ctx, id)
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
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	row, err := lockedRow(ctx, q, id)
	if err != nil {
		return err
	}
	if err = s.requireClosed(ctx, row); err != nil {
		return err
	}
	if err = guardActivityForm(ctx, q, row, dbgen.UpdateFormParams{}); err != nil {
		return err
	}
	if err = q.DeleteForm(ctx, row.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s Service) Toggle(ctx context.Context, id string) (Response, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Response{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	row, err := lockedRow(ctx, q, id)
	if err != nil {
		return Response{}, err
	}
	active := row.IsActive != nil && *row.IsActive
	if active {
		if err = s.requireClosed(ctx, row); err != nil {
			return Response{}, err
		}
	}
	next, err := merged(row, Input{})
	if err != nil {
		return Response{}, err
	}
	nextActive := !active
	next.IsActive = &nextActive
	if err = guardActivityForm(ctx, q, row, next); err != nil {
		return Response{}, err
	}
	updated, err := q.ToggleFormActive(ctx, dbgen.ToggleFormActiveParams{ID: row.ID, IsActive: !active})
	if err != nil {
		return Response{}, err
	}
	return View(updated, s.Location), tx.Commit(ctx)
}
func (s Service) AttachActivity(ctx context.Context, id string, input ActivityAttachment) (ActivityAttached, error) {
	if !database.JSONTruthy(input.ActivityID) {
		return ActivityAttached{}, domain.Fail(400, "ACTIVITY_ID_REQUIRED")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return ActivityAttached{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	row, err := lockedRow(ctx, q, id)
	if err != nil {
		return ActivityAttached{}, err
	}
	if row.FeatureID != nil && *row.FeatureID != 0 {
		return ActivityAttached{}, domain.Fail(400, "FORM_ALREADY_ATTACHED")
	}
	params, err := merged(row, Input{})
	if err != nil {
		return ActivityAttached{}, err
	}
	kind, activityID := "activity_registration", database.JSONParameter(input.ActivityID)
	params.FeatureType, params.FeatureID = &kind, &activityID
	if err = guardActivityForm(ctx, q, row, params); err != nil {
		return ActivityAttached{}, err
	}
	next, err := q.AttachFormToActivity(ctx, dbgen.AttachFormToActivityParams{ID: row.ID, Identifier: activityID})
	statement := `update "custom_forms" set "feature_id" = $1, "updated_at" = $2, "feature_type" = $3 where "id" = $4`
	if row.FeatureType != nil && *row.FeatureType == "activity_registration" {
		statement = `update "custom_forms" set "feature_id" = $1, "updated_at" = $2 where "id" = $3`
	}
	if err != nil {
		return ActivityAttached{}, database.LegacyQueryError(err, statement)
	}
	return ActivityAttached{Response: View(next, s.Location), FeatureID: input.ActivityID}, tx.Commit(ctx)
}
func (s Service) DetachActivity(ctx context.Context, id string) (Response, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Response{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	row, err := lockedRow(ctx, q, id)
	if err != nil {
		return Response{}, err
	}
	if err = guardActivityForm(ctx, q, row, dbgen.UpdateFormParams{}); err != nil {
		return Response{}, err
	}
	row, err = q.DetachFormFromActivity(ctx, row.ID)
	if err != nil {
		return Response{}, err
	}
	return View(row, s.Location), tx.Commit(ctx)
}
