package club

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
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
	row, err := q.LockClubByIdentifier(ctx, identifier)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "CLUB_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if confirmation == "" || confirmation != row.Name {
		return domain.Fail(422, "DELETE_CONFIRMATION_MISMATCH")
	}
	if err = q.DeleteClub(ctx, row.ID); err != nil {
		return err
	}
	if err = q.DetachDeletedFeatureForms(ctx, dbgen.DetachDeletedFeatureFormsParams{FeatureType: "club_registration", FeatureID: row.ID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
