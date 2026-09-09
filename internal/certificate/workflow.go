package certificate

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"math"
	"strings"
	"time"
)

type Failure struct {
	RegistrationID int64  `json:"registration_id"`
	Reason         string `json:"reason"`
}
type BulkResult struct {
	RemainingIDs       []int64    `json:"remaining_ids"`
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

func UniqueIDs[T ~int32 | ~int64](ids []T) []T {
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
func (s Issuance) Bulk(ctx context.Context, ids []int64, actor *int32, requestID string, expected *Expectation) (BulkResult, error) {
	started := time.Now()
	ids = UniqueIDs(ids)
	out := BulkResult{RemainingIDs: []int64{}, Created: []Response{}, AlreadyIssued: []Response{}, Issued: []Response{}, Skipped: []Failure{}, Failed: []Failure{}, TotalRequested: len(ids)}
	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if id > math.MaxInt32 {
			out.Failed = append(out.Failed, Failure{id, "GENERAL_ERROR"})
			continue
		}
		result, err := s.Issue(ctx, int32(id), actor, requestID, expected)
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
func (s Issuance) ByID(ctx context.Context, id int32) (Response, error) {
	row, err := dbgen.New(s.Pool).IssuedByID(ctx, id)
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
func (s Issuance) Revoke(ctx context.Context, id int32, reason string, actor int32, requestID string) (Response, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Response{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	row, err := q.LockIssued(ctx, id)
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
	row, err = q.RevokeIssued(ctx, dbgen.RevokeIssuedParams{ID: id, RevokedAt: now, RevokedReason: &reason, RevokedBy: &actor})
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

const recipientName = "COALESCE((SELECT NULLIF(p.name,'') FROM profiles p WHERE p.user_id=r.user_id ORDER BY p.id LIMIT 1),NULLIF(r.guest_data->>'name',''),'Peserta')"
const recipientState = "CASE WHEN c.revoked_at IS NOT NULL THEN 'issued_revoked' WHEN c.id IS NOT NULL THEN 'issued_active' WHEN r.status='LULUS KEGIATAN' THEN 'eligible_not_issued' ELSE 'not_eligible' END"
const recipientFrom = " FROM activity_registrations r LEFT JOIN issued_certificates c ON c.registration_id=r.id WHERE r.activity_id=$1"

type RecipientOptions struct {
	Page, PerPage            int
	SortOrder, Search, State string
	RegistrationIDs          []int32
}
type Recipients struct {
	Activity database.Object        `json:"activity"`
	Template database.Object        `json:"template"`
	Counts   map[string]int32       `json:"counts"`
	Meta     database.RawPagination `json:"meta"`
	Data     []database.Object      `json:"data"`
}

func (s Issuance) Recipients(ctx context.Context, id int32, options RecipientOptions) (Recipients, error) {
	q := database.JSONQueries{DB: s.Pool}
	activity, err := q.One(ctx, "SELECT * FROM activities WHERE id=$1", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Recipients{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return Recipients{}, err
	}
	query := "SELECT r.id AS registration_id,r.created_at,r.status,c.id AS certificate_id,c.certificate_code," + recipientName + " AS name," + recipientState + " AS state" + recipientFrom
	args := []interface{}{id}
	if options.Search != "" {
		args = append(args, "%"+options.Search+"%")
		query += fmt.Sprintf(" AND %s ILIKE $%d", recipientName, len(args))
	}
	if options.State != "" {
		args = append(args, options.State)
		query += fmt.Sprintf(" AND %s=$%d", recipientState, len(args))
	}
	if options.RegistrationIDs != nil {
		args = append(args, options.RegistrationIDs)
		query += fmt.Sprintf(" AND r.id=ANY($%d::int[])", len(args))
	}
	direction := "DESC"
	if options.SortOrder == "asc" {
		direction = "ASC"
	}
	query += " ORDER BY r.created_at " + direction + " NULLS LAST,r.id " + direction
	page, err := q.Paginate(ctx, query, args, float64(options.Page), float64(options.PerPage))
	if err != nil {
		return Recipients{}, err
	}
	countRows, err := q.All(ctx, "SELECT "+recipientState+" AS state,count(*)::int AS total"+recipientFrom+" GROUP BY "+recipientState, id)
	if err != nil {
		return Recipients{}, err
	}
	counts := map[string]int32{"eligible_not_issued": 0, "not_eligible": 0, "issued_active": 0, "issued_revoked": 0}
	for _, row := range countRows {
		counts[row.String("state")] = row.ID("total")
	}
	var template database.Object
	if templateID := ActivityTemplateID(activity); templateID != 0 {
		row, err := q.One(ctx, "SELECT * FROM certificate_templates WHERE id=$1", templateID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Recipients{}, err
		}
		if row != nil {
			template = TemplateSummary(row)
			delete(template, "description")
		}
	}
	a := database.Object{}
	a.Set("id", id)
	a.Set("name", activity.String("name"))
	for _, row := range page.Data {
		database.Timestamps(row, time.UTC, "created_at")
	}
	return Recipients{Activity: a, Template: template, Counts: counts, Meta: page.Meta.Raw(), Data: page.Data}, nil
}

type Excluded struct {
	AlreadyIssued int `json:"already_issued"`
	Revoked       int `json:"revoked"`
	NotEligible   int `json:"not_eligible"`
	Missing       int `json:"missing"`
}
type Prepared struct {
	ActivityID      int32     `json:"activity_id"`
	TemplateID      int32     `json:"template_id"`
	TemplateVersion int32     `json:"template_version"`
	RegistrationIDs []int32   `json:"registration_ids"`
	Excluded        Excluded  `json:"excluded"`
	Preview         *Response `json:"preview"`
}

func (s Issuance) Prepare(ctx context.Context, id int32, ids []int32) (Prepared, error) {
	q := database.JSONQueries{DB: s.Pool}
	activity, err := q.One(ctx, "SELECT * FROM activities WHERE id=$1", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Prepared{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return Prepared{}, err
	}
	templateID := ActivityTemplateID(activity)
	template, err := q.One(ctx, "SELECT * FROM certificate_templates WHERE id=$1", templateID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Prepared{}, err
	}
	if template == nil || template.String("lifecycle_status") != "published" || !CheckReadiness(template).Ready {
		return Prepared{}, Error("CERTIFICATE_TEMPLATE_NOT_READY")
	}
	query := "SELECT r.id AS registration_id," + recipientState + " AS state" + recipientFrom
	args := []interface{}{id}
	if ids != nil {
		args = append(args, ids)
		query += " AND r.id=ANY($2::int[])"
	}
	rows, err := q.All(ctx, query+" ORDER BY r.id ASC", args...)
	if err != nil {
		return Prepared{}, err
	}
	out := Prepared{ActivityID: id, TemplateID: templateID, TemplateVersion: template.ID("version"), RegistrationIDs: []int32{}}
	for _, row := range rows {
		switch row.String("state") {
		case "eligible_not_issued":
			out.RegistrationIDs = append(out.RegistrationIDs, row.ID("registration_id"))
		case "issued_active":
			out.Excluded.AlreadyIssued++
		case "issued_revoked":
			out.Excluded.Revoked++
		case "not_eligible":
			out.Excluded.NotEligible++
		}
	}
	if ids != nil {
		out.Excluded.Missing = len(UniqueIDs(ids)) - len(rows)
	}
	if len(out.RegistrationIDs) > 0 {
		preview, err := s.Preview(ctx, out.RegistrationIDs[0])
		if err != nil {
			return Prepared{}, err
		}
		if preview.Template.ID != templateID || preview.Template.Version != template.ID("version") {
			return Prepared{}, Error("CERTIFICATE_CONTEXT_CHANGED")
		}
		out.Preview = &preview
	}
	return out, nil
}
func (s Issuance) RecipientNames(ctx context.Context, ids []int32) (map[int32]string, error) {
	names := map[int32]string{}
	if len(ids) == 0 {
		return names, nil
	}
	rows, err := (database.JSONQueries{DB: s.Pool}).All(ctx, "SELECT r.id,"+recipientName+" AS name FROM activity_registrations r WHERE r.id=ANY($1::int[])", ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		names[row.ID("id")] = row.String("name")
	}
	return names, nil
}

func (s Issuance) List(ctx context.Context, activityID int32, ids []int32, page, size float64) (database.Page, error) {
	q := database.JSONQueries{DB: s.Pool}
	query := "SELECT c.id,c.certificate_code,c.registration_id,c.activity_id,c.participant_snapshot->>'name' AS participant_name,c.participant_snapshot->>'email' AS participant_email,c.participant_snapshot->>'activity_name' AS activity_name,COALESCE(c.template_snapshot->>'name','') AS template_name,c.issued_at,c.issued_by,issuer.display_name AS issued_by_name,c.revoked_at,c.revoked_reason,c.revoked_by,revoker.display_name AS revoked_by_name,CASE WHEN c.revoked_at IS NULL THEN 'issued_active' ELSE 'issued_revoked' END AS state FROM issued_certificates c LEFT JOIN admin_users issuer ON issuer.id=c.issued_by LEFT JOIN admin_users revoker ON revoker.id=c.revoked_by WHERE true"
	args := []interface{}{}
	if activityID != 0 {
		args = append(args, activityID)
		query += fmt.Sprintf(" AND c.activity_id=$%d", len(args))
	}
	if ids != nil {
		args = append(args, ids)
		query += fmt.Sprintf(" AND c.registration_id=ANY($%d::int[])", len(args))
	}
	result, err := q.Paginate(ctx, query+" ORDER BY c.issued_at DESC", args, page, size)
	if err != nil {
		return result, err
	}
	for _, row := range result.Data {
		for _, key := range []string{"issued_at", "revoked_at"} {
			if value := row.String(key); value != "" {
				if date, err := time.Parse(time.RFC3339Nano, value); err == nil {
					row.Set(key, ISO(date, s.Location))
				}
			}
		}
	}
	return result, nil
}

type BulkPreview struct {
	Activity     database.Object `json:"activity"`
	Template     database.Object `json:"template"`
	Participants []Participant   `json:"participants"`
	Total        int             `json:"total"`
}

func (s Issuance) PreviewBulk(ctx context.Context, id int32, status string) (BulkPreview, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return BulkPreview{}, err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	a, err := q.One(ctx, "SELECT * FROM activities WHERE id=$1", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return BulkPreview{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return BulkPreview{}, err
	}
	templateID := ActivityTemplateID(a)
	if templateID == 0 {
		return BulkPreview{}, Error("NO_CERTIFICATE_TEMPLATE")
	}
	template, err := q.One(ctx, "SELECT * FROM certificate_templates WHERE id=$1", templateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return BulkPreview{}, Error("CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return BulkPreview{}, err
	}
	if template.String("lifecycle_status") != "published" {
		return BulkPreview{}, Error("CERTIFICATE_TEMPLATE_NOT_PUBLISHED")
	}
	if ready := CheckReadiness(template); !ready.Ready {
		return BulkPreview{}, Error("CERTIFICATE_TEMPLATE_NOT_READY", ready.Errors)
	}
	rows, err := q.All(ctx, "SELECT * FROM activity_registrations WHERE activity_id=$1 AND status=$2", id, status)
	if err != nil {
		return BulkPreview{}, err
	}
	if len(rows) == 0 {
		return BulkPreview{}, Error("INVALID_STATUS")
	}
	database.Timestamps(a, s.Location, "created_at", "updated_at")
	database.Timestamps(template, s.Location, "created_at", "updated_at", "published_at", "archived_at")
	out := BulkPreview{Activity: a, Template: template, Participants: []Participant{}, Total: len(rows)}
	for _, row := range rows {
		p, err := s.participant(ctx, q, row, a)
		if err != nil {
			return BulkPreview{}, err
		}
		out.Participants = append(out.Participants, p)
	}
	return out, tx.Commit(ctx)
}
