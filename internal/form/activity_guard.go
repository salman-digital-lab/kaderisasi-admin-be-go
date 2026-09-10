package form

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"reflect"
	"slices"
)

func attachedActivity(kind, id *string) *string {
	if kind == nil || *kind != "activity_registration" {
		return nil
	}
	return id
}

// Lock activities before changing their form so publication and form edits
// cannot race. Valid live schema edits remain available to activity editors.
func guardActivityForm(ctx context.Context, q *dbgen.Queries, old dbgen.CustomForm, next dbgen.UpdateFormParams) error {
	oldID := attachedActivity(old.FeatureType, storedNumber(old.FeatureID))
	newID := attachedActivity(next.FeatureType, next.FeatureID)
	ids := []string{}
	if oldID != nil {
		ids = append(ids, *oldID)
	}
	if newID != nil {
		ids = append(ids, *newID)
	}
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		row, err := q.LockActivityByIdentifier(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Fail(404, "ACTIVITY_NOT_FOUND")
		}
		if err != nil {
			return err
		}
		if oldID != nil && *oldID == id && row.IsRegistrationOpen && old.IsActive != nil && *old.IsActive {
			if !reflect.DeepEqual(oldID, newID) || next.IsActive == nil || !*next.IsActive || !ValidSchema(next.FormSchema) {
				return domain.Fail(400, "CLOSE_REGISTRATION_BEFORE_FORM_CHANGE")
			}
		}
	}
	if newID != nil {
		if next.IsActive != nil && *next.IsActive && !ValidSchema(next.FormSchema) {
			return domain.Fail(422, "INVALID_ACTIVITY_FORM_SCHEMA")
		}
		if !reflect.DeepEqual(oldID, newID) {
			exists, err := q.OtherActivityFormExists(ctx, dbgen.OtherActivityFormExistsParams{Identifier: *newID, FormID: old.ID})
			if err != nil {
				return err
			}
			if exists {
				return domain.Fail(409, "ACTIVITY_ALREADY_HAS_FORM")
			}
		}
	}
	return nil
}
