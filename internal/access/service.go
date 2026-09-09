package access

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

type Update struct {
	RoleCode domain.Optional[string]
	IsActive domain.Optional[bool]
}

// Change runs inside the caller's transaction after advisory lock (7411,1).
func Change(ctx context.Context, tx pgx.Tx, userID int32, change Update) error {
	q := dbgen.New(tx)
	user, err := q.LockAdmin(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "USER_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if change.RoleCode.Present && change.RoleCode.Value != nil && auth.RoleByCode(*change.RoleCode.Value) == nil {
		return domain.Fail(409, "UNKNOWN_ROLE")
	}
	deactivate := change.IsActive.Present && change.IsActive.Value != nil && !*change.IsActive.Value
	removeSuper := change.RoleCode.Present && (change.RoleCode.Value == nil || *change.RoleCode.Value != "super_admin")
	if user.IsActive && user.RoleCode != nil && *user.RoleCode == "super_admin" && (deactivate || removeSuper) {
		total, err := q.CountActiveSuperAdmins(ctx)
		if err != nil {
			return err
		}
		if total <= 1 {
			return domain.Fail(409, "LAST_SUPER_ADMIN_REQUIRED")
		}
	}
	active := user.IsActive
	if change.IsActive.Value != nil {
		active = *change.IsActive.Value
	}
	if err = q.UpdateAdminAccess(ctx, dbgen.UpdateAdminAccessParams{ID: userID, RolePresent: change.RoleCode.Present, RoleCode: change.RoleCode.Value, ActivePresent: change.IsActive.Present, IsActive: active}); err != nil {
		return err
	}
	if deactivate {
		reason := "account_deactivated"
		return dbgen.New(tx).RevokeAdminSessions(ctx, dbgen.RevokeAdminSessionsParams{AdminUserID: userID, RevocationReason: &reason})
	}
	return nil
}
