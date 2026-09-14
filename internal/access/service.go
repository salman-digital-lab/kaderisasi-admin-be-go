package access

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"slices"
)

type Update struct {
	RoleCode  domain.Optional[string]
	RoleCodes domain.Optional[[]string]
	IsActive  domain.Optional[bool]
}

func ResolveRoles(primary *string, roles domain.Optional[[]string]) (*string, []string, error) {
	codes := auth.RoleCodes(primary, nil)
	if roles.Present {
		if roles.Value == nil {
			return nil, nil, domain.Fail(422, "INVALID_ROLE_CODES")
		}
		codes = auth.RoleCodes(nil, *roles.Value)
	}
	for _, code := range codes {
		if auth.RoleByCode(code) == nil {
			return nil, nil, domain.Fail(409, "UNKNOWN_ROLE")
		}
	}
	if len(codes) == 0 {
		return nil, []string{}, nil
	}
	return &codes[0], codes[1:], nil
}

// Change runs inside the caller's transaction after advisory lock (7411,1).
func Change(ctx context.Context, tx pgx.Tx, identifier string, change Update) error {
	q := dbgen.New(tx)
	user, err := q.LockAdminByIdentifier(ctx, identifier)
	err = database.LegacyQueryError(err, `select * from "admin_users" where "id" = $1 limit $2 for update`)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "USER_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if change.RoleCode.Present && change.RoleCodes.Present {
		return domain.Fail(422, "CONFLICTING_ROLE_FIELDS")
	}
	primary, additional, err := ResolveRoles(change.RoleCode.Value, change.RoleCodes)
	if err != nil {
		return err
	}
	rolePresent := change.RoleCode.Present || change.RoleCodes.Present
	deactivate := change.IsActive.Present && change.IsActive.Value != nil && !*change.IsActive.Value
	removeSuper := rolePresent && !slices.Contains(auth.RoleCodes(primary, additional), "super_admin")
	if auth.ForUser(user).IsSuperAdmin && (deactivate || removeSuper) {
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
	if err = q.UpdateAdminAccess(ctx, dbgen.UpdateAdminAccessParams{ID: user.ID, RolePresent: rolePresent, RoleCode: primary, AdditionalRoleCodes: additional, ActivePresent: change.IsActive.Present, IsActive: active}); err != nil {
		return err
	}
	if deactivate {
		reason := "account_deactivated"
		return dbgen.New(tx).RevokeAdminSessions(ctx, dbgen.RevokeAdminSessionsParams{AdminUserID: user.ID, RevocationReason: &reason})
	}
	return nil
}
