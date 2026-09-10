package access

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"strconv"
	"strings"
	"time"
)

var ErrTicketAccess = errors.New("ticket access denied")

type Tickets struct{ Pool *pgxpool.Pool }

func (service Tickets) Create(ctx context.Context, actor int32, code, reason string) (int32, error) {
	role := auth.RoleByCode(code)
	if role == nil || !role.IsRequestable {
		return 0, domain.Fail(422, "ROLE_NOT_REQUESTABLE")
	}
	tx, err := service.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if err = q.AcquireRequestLock(ctx, actor); err != nil {
		return 0, err
	}
	_, err = q.OpenRoleRequest(ctx, dbgen.OpenRoleRequestParams{RequesterAdminUserID: actor, RequestedRoleCode: role.Code})
	if err == nil {
		return 0, domain.Fail(409, "DUPLICATE_OPEN_REQUEST")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, err
	}
	random := make([]byte, 3)
	if _, err = rand.Read(random); err != nil {
		return 0, err
	}
	number := "AR-" + strings.ToUpper(strconv.FormatInt(time.Now().UnixMilli(), 36)) + "-" + strings.ToUpper(hex.EncodeToString(random))
	id, err := q.CreateTicket(ctx, dbgen.CreateTicketParams{Number: number, RequesterAdminUserID: actor, RequestedRoleCode: role.Code, Reason: reason})
	if err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}

func (service Tickets) Cancel(ctx context.Context, id string, actor int32) error {
	ticket, err := dbgen.New(service.Pool).TicketByIdentifier(ctx, id)
	err = database.LegacyQueryError(err, `select * from "tickets" where "id" = $1 limit $2`)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "TICKET_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if ticket.RequesterAdminUserID != actor || ticket.Status != "open" {
		return ErrTicketAccess
	}
	tx, err := service.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	locked, err := q.LockTicketByIdentifier(ctx, id)
	if err != nil {
		return err
	}
	if locked.Status != "open" {
		return domain.Fail(409, "TICKET_ALREADY_TERMINAL")
	}
	if err = q.CancelTicket(ctx, ticket.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (service Tickets) Resolve(ctx context.Context, id string, actor int32, resolution string, reason *string) error {
	if resolution != "approved" && resolution != "rejected" {
		return errors.New("invalid ticket resolution")
	}
	tx, err := service.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if err = q.AcquireRBACLock(ctx); err != nil {
		return err
	}
	reviewer, err := q.FindAdminByID(ctx, actor)
	if err != nil {
		return err
	}
	if !auth.ForRole(reviewer.RoleCode, reviewer.IsActive).Allows("tickets.review") {
		return domain.Fail(403, "FORBIDDEN")
	}
	ticket, err := q.LockTicketByIdentifier(ctx, id)
	err = database.LegacyQueryError(err, `select * from "tickets" where "id" = $1 limit $2 for update`)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "TICKET_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if ticket.RequesterAdminUserID == reviewer.ID {
		return domain.Fail(409, "SELF_REVIEW_NOT_ALLOWED")
	}
	if ticket.Status != "open" {
		return domain.Fail(409, "TICKET_ALREADY_TERMINAL")
	}
	if resolution == "approved" {
		role := auth.RoleByCode(ticket.RequestedRoleCode)
		if role == nil || !role.IsRequestable {
			return domain.Fail(409, "ROLE_NOT_REQUESTABLE")
		}
		if err = Change(ctx, tx, strconv.FormatInt(int64(ticket.RequesterAdminUserID), 10), Update{RoleCode: domain.Value(role.Code)}); err != nil {
			return err
		}
	}
	if err = q.ResolveTicket(ctx, dbgen.ResolveTicketParams{ID: ticket.ID, Resolution: &resolution, RejectionReason: reason, ResolvedByAdminUserID: &actor}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
