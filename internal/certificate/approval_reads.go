package certificate

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

type ApprovalPage struct {
	Data []dbgen.ListCertificateApprovalsRow `json:"data"`
	Meta database.Pagination                 `json:"meta"`
}

func (s Issuance) Approvals(ctx context.Context, actor int32, activityID *int32, status *string, page, size float64) (ApprovalPage, error) {
	q := dbgen.New(s.Pool)
	total, err := q.CountCertificateApprovals(ctx, dbgen.CountCertificateApprovalsParams{ActorID: actor, ActivityID: activityID, Status: status})
	out := ApprovalPage{Data: []dbgen.ListCertificateApprovalsRow{}, Meta: database.Meta(total, page, size)}
	if err != nil {
		return out, err
	}
	limit, offset, err := database.SQLPage(page, size)
	if err != nil {
		return out, err
	}
	out.Data, err = q.ListCertificateApprovals(ctx, dbgen.ListCertificateApprovalsParams{ActorID: actor, ActivityID: activityID, Status: status, PageSize: limit, PageOffset: offset})
	return out, err
}
func (s Issuance) Approval(ctx context.Context, id, actor int32) (dbgen.CertificateApproval, error) {
	row, err := dbgen.New(s.Pool).CertificateApprovalByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, domain.Fail(404, "APPROVAL_NOT_FOUND")
	}
	if err == nil && row.SignerID != actor && row.RequestedBy != actor {
		return dbgen.CertificateApproval{}, domain.Fail(404, "APPROVAL_NOT_FOUND")
	}
	return row, err
}
