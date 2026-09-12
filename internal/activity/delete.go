package activity

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

func (s Service) Delete(ctx context.Context, identifier, confirmation string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	q := dbgen.New(tx)
	row, err := q.LockActivityByIdentifier(ctx, identifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if confirmation == "" || confirmation != row.Name {
		return domain.Fail(422, "DELETE_CONFIRMATION_MISMATCH")
	}
	if _, err = q.DeleteActivity(ctx, row.ID); err != nil {
		var constraint *pgconn.PgError
		if errors.As(err, &constraint) && constraint.Code == "23503" {
			return domain.Fail(409, "ACTIVITY_HAS_CERTIFICATE_HISTORY")
		}
		return err
	}
	if err = q.DetachDeletedFeatureForms(ctx, dbgen.DetachDeletedFeatureFormsParams{FeatureType: "activity_registration", FeatureID: row.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
