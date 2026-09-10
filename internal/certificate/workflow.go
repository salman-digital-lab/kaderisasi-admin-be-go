package certificate

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"strings"
	"time"
)

type Failure struct {
	RegistrationID float64 `json:"registration_id"`
	Reason         string  `json:"reason"`
}
type BulkResult struct {
	RemainingIDs       []float64  `json:"remaining_ids"`
	Paused             bool       `json:"paused"`
	Created            []Response `json:"created"`
	AlreadyIssued      []Response `json:"already_issued"`
	Issued             []Response `json:"issued"`
	Skipped            []Failure  `json:"skipped"`
	Failed             []Failure  `json:"failed"`
	TotalRequested     int        `json:"total_requested"`
	TotalCreated       int        `json:"total_created"`
	TotalAlreadyIssued int        `json:"total_already_issued"`
	TotalSkipped       int        `json:"total_skipped"`
	TotalFailed        int        `json:"total_failed"`
}

func UniqueIDs[T ~int32 | ~int64 | ~float64](ids []T) []T {
	out := []T{}
	seen := map[T]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
func (s Issuance) Bulk(ctx context.Context, ids []float64, actor *int32, requestID string, expected *Expectation) (BulkResult, error) {
	started := time.Now()
	ids = UniqueIDs(ids)
	out := BulkResult{RemainingIDs: []float64{}, Created: []Response{}, AlreadyIssued: []Response{}, Issued: []Response{}, Skipped: []Failure{}, Failed: []Failure{}, TotalRequested: len(ids)}
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		result, err := s.Issue(ctx, id, actor, requestID, expected)
		if err != nil {
			var d *domain.Error
			if errors.As(err, &d) {
				if d.Message == "CERTIFICATE_CONTEXT_CHANGED" {
					out.RemainingIDs = ids[i:]
					break
				}
				out.Skipped = append(out.Skipped, Failure{id, d.Message})
			} else {
				out.Failed = append(out.Failed, Failure{id, "GENERAL_ERROR"})
			}
		} else if result.Created {
			out.Created = append(out.Created, result.Data)
		} else {
			out.AlreadyIssued = append(out.AlreadyIssued, result.Data)
		}
	}
	out.Paused = len(out.RemainingIDs) > 0
	out.Issued = out.Created
	out.TotalCreated = len(out.Created)
	out.TotalAlreadyIssued = len(out.AlreadyIssued)
	out.TotalSkipped = len(out.Skipped)
	out.TotalFailed = len(out.Failed)
	if s.Logger != nil {
		s.Logger.Info("certificate_bulk_issue_completed", "event", "certificate_bulk_issue_completed", "request_id", requestID, "actor_admin_id", actor, "duration_ms", time.Since(started).Milliseconds(), "context_changed", out.Paused, "total_requested", out.TotalRequested, "total_created", out.TotalCreated, "total_already_issued", out.TotalAlreadyIssued, "total_skipped", out.TotalSkipped, "total_failed", out.TotalFailed)
	}
	return out, nil
}
func (s Issuance) ByID(ctx context.Context, id float64) (Response, error) {
	row, err := dbgen.New(s.Pool).IssuedCertificateByIdentifier(ctx, database.JSNumber(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Response{}, Error("CERTIFICATE_NOT_FOUND")
	}
	if err != nil {
		return Response{}, err
	}
	return s.IssuedResponse(row)
}
func (s Issuance) ByCode(ctx context.Context, code string) (Response, error) {
	row, err := dbgen.New(s.Pool).IssuedByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Response{}, Error("CERTIFICATE_NOT_FOUND")
	}
	if err != nil {
		return Response{}, err
	}
	return s.IssuedResponse(row)
}
func (s Issuance) Revoke(ctx context.Context, id float64, reason string, actor int32, requestID string) (Response, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Response{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	row, err := q.LockIssuedCertificateByIdentifier(ctx, database.JSNumber(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Response{}, Error("CERTIFICATE_NOT_FOUND")
	}
	if err != nil {
		return Response{}, err
	}
	if row.RevokedAt.Valid {
		return Response{}, Error("CERTIFICATE_ALREADY_REVOKED")
	}
	now := pgtype.Timestamptz{Time: time.Now().Truncate(time.Millisecond), Valid: true}
	row, err = q.RevokeIssued(ctx, dbgen.RevokeIssuedParams{ID: row.ID, RevokedAt: now, RevokedReason: &reason, RevokedBy: &actor})
	if err != nil {
		return Response{}, err
	}
	data, err := s.IssuedResponse(row)
	if err != nil {
		return Response{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Response{}, err
	}
	if s.Logger != nil {
		s.Logger.Warn("certificate_revoked", "event", "certificate_revoked", "request_id", requestID, "actor_admin_id", actor, "certificate_id", id, "reason", reason)
	}
	return data, nil
}
