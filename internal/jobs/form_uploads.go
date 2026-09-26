package jobs

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/dbgen"
)

func (r Runner) cleanFormUpload(ctx context.Context, file dbgen.CustomFormAttachment) (bool, error) {
	beginner, ok := r.DB.(interface {
		Begin(context.Context) (pgx.Tx, error)
	})
	if !ok {
		return false, fmt.Errorf("form upload cleanup requires transactions")
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	q := dbgen.New(tx)
	if _, err = q.LockCleanupFormSession(ctx, file.SessionID); errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	current, err := q.LockExpiredFormAttachment(ctx, dbgen.LockExpiredFormAttachmentParams{ID: file.ID, SessionID: file.SessionID, StorageKey: file.StorageKey})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = r.Storage.Delete(ctx, current.StorageKey); err != nil {
		return false, err
	}
	if err = q.DeleteExpiredFormAttachment(ctx, current.ID); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
