package certificate

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

type ApprovalEvidence struct {
	RequestID   int32  `json:"request_id"`
	SignerID    int32  `json:"signer_id"`
	SignerName  string `json:"signer_name"`
	SignerTitle string `json:"signer_title"`
	ApprovedAt  string `json:"approved_at"`
	ContentHash string `json:"content_hash"`
}
type Signer struct {
	ID   int32  `json:"id"`
	Name string `json:"name"`
}
type ApprovalRequestInput struct {
	RegistrationIDs []int32     `json:"registration_ids"`
	SignerID        int32       `json:"signer_id"`
	SignerTitle     string      `json:"signer_title"`
	Expected        Expectation `json:"expected"`
}
type ApprovalDecisionItem struct {
	ID          int32  `json:"id"`
	ContentHash string `json:"content_hash"`
}
type ApprovalDecisionInput struct {
	Items   []ApprovalDecisionItem `json:"items"`
	Action  string                 `json:"action"`
	Reason  string                 `json:"reason"`
	Consent bool                   `json:"consent"`
}
type ApprovalOutcome struct {
	ID             int32  `json:"id"`
	RegistrationID int32  `json:"registration_id"`
	Status         string `json:"status"`
	Reason         string `json:"reason,omitempty"`
	CertificateID  *int32 `json:"certificate_id,omitempty"`
}

func ApprovalHash(data Response, signerID int32, signerName, signerTitle string) (string, error) {
	raw, err := json.Marshal(struct {
		Data                    Response
		SignerID                int32
		SignerName, SignerTitle string
	}{data, signerID, signerName, signerTitle})
	if err != nil {
		return "", err
	}
	// JSONB reorders object keys. Canonicalize the JSON document before hashing.
	var document interface{}
	if err = json.Unmarshal(raw, &document); err != nil {
		return "", err
	}
	raw, err = json.Marshal(document)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:]), nil
}
func (s Issuance) Signers(ctx context.Context) ([]Signer, error) {
	rows, err := dbgen.New(s.Pool).CertificateSigners(ctx)
	result := []Signer{}
	if err != nil {
		return result, err
	}
	for _, row := range rows {
		if row.DisplayName != nil && strings.TrimSpace(*row.DisplayName) != "" && auth.ForRole(row.RoleCode, row.IsActive).Allows("certificate.approve") {
			result = append(result, Signer{row.ID, *row.DisplayName})
		}
	}
	return result, nil
}
func approvalFailure(id, registrationID int32, err error) ApprovalOutcome {
	result := ApprovalOutcome{ID: id, RegistrationID: registrationID, Status: "failed", Reason: "GENERAL_ERROR"}
	var d *domain.Error
	if errors.As(err, &d) {
		result.Reason = d.Message
	}
	return result
}
func (s Issuance) RequestApprovals(ctx context.Context, input ApprovalRequestInput, actor int32) ([]ApprovalOutcome, error) {
	input.SignerTitle = strings.TrimSpace(input.SignerTitle)
	ids := UniqueIDs(input.RegistrationIDs)
	if len(ids) == 0 || len(ids) > 50 || input.SignerID <= 0 || input.SignerTitle == "" || utf8.RuneCountInString(input.SignerTitle) > 120 || input.Expected.ActivityID <= 0 || input.Expected.TemplateID <= 0 || input.Expected.TemplateVersion <= 0 {
		return nil, domain.Fail(422, "INVALID_APPROVAL_REQUEST")
	}
	for _, id := range ids {
		if id <= 0 {
			return nil, domain.Fail(422, "INVALID_APPROVAL_REQUEST")
		}
	}
	result := []ApprovalOutcome{}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		row, err := s.requestApproval(ctx, id, input, actor)
		if err != nil {
			result = append(result, approvalFailure(0, id, err))
		} else {
			result = append(result, ApprovalOutcome{ID: row.ID, RegistrationID: id, Status: row.Status})
		}
	}
	return result, nil
}
func (s Issuance) requestApproval(ctx context.Context, id int32, input ApprovalRequestInput, actor int32) (dbgen.CertificateApproval, error) {
	var empty dbgen.CertificateApproval
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if err = s.checkExpected(ctx, q, float64(id), input.Expected); err != nil {
		return empty, err
	}
	source, err := s.load(ctx, q, float64(id), true)
	if err != nil {
		return empty, err
	}
	if !RequiresApproval(source.Template.TemplateData) {
		return empty, domain.Fail(422, "APPROVAL_BLOCK_REQUIRED")
	}
	if _, err = q.IssuedByRegistration(ctx, id); err == nil {
		return empty, domain.Fail(409, "CERTIFICATE_ALREADY_ISSUED")
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return empty, err
	}
	signer, err := q.FindAdminByID(ctx, input.SignerID)
	if err != nil {
		return empty, domain.Fail(422, "INVALID_CERTIFICATE_SIGNER")
	}
	if !auth.ForRole(signer.RoleCode, signer.IsActive).Allows("certificate.approve") || signer.DisplayName == nil || strings.TrimSpace(*signer.DisplayName) == "" {
		return empty, domain.Fail(422, "INVALID_CERTIFICATE_SIGNER")
	}
	hash, err := ApprovalHash(source.Data, signer.ID, *signer.DisplayName, input.SignerTitle)
	if err != nil {
		return empty, err
	}
	pending, err := q.PendingCertificateApproval(ctx, id)
	if err == nil {
		if pending.ContentHash != hash {
			return empty, domain.Fail(409, "CERTIFICATE_APPROVAL_PENDING")
		}
		return pending, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return empty, err
	}
	snapshot, err := json.Marshal(source.Data)
	if err != nil {
		return empty, err
	}
	row, err := q.InsertCertificateApproval(ctx, dbgen.InsertCertificateApprovalParams{RegistrationID: id, ActivityID: source.Activity.ID, SignerID: signer.ID, RequestedBy: actor, SignerName: *signer.DisplayName, SignerTitle: input.SignerTitle, Snapshot: snapshot, ContentHash: hash})
	if err != nil {
		return empty, err
	}
	return row, tx.Commit(ctx)
}
func (s Issuance) DecideApprovals(ctx context.Context, input ApprovalDecisionInput, actor int32) ([]ApprovalOutcome, error) {
	if input.Action == "reject" && strings.TrimSpace(input.Reason) == "" {
		return nil, domain.Fail(422, "APPROVAL_REJECTION_REASON_REQUIRED")
	}
	if len(input.Items) == 0 || len(input.Items) > 50 || (input.Action != "approve" && input.Action != "reject" && input.Action != "cancel") || (input.Action == "approve" && !input.Consent) || utf8.RuneCountInString(input.Reason) > 500 {
		return nil, domain.Fail(422, "INVALID_APPROVAL_DECISION")
	}
	seen := map[int32]bool{}
	for _, item := range input.Items {
		if item.ID <= 0 || len(item.ContentHash) != 64 || seen[item.ID] {
			return nil, domain.Fail(422, "INVALID_APPROVAL_DECISION")
		}
		seen[item.ID] = true
	}
	result := []ApprovalOutcome{}
	for _, item := range input.Items {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		row, err := s.decideApproval(ctx, item, input, actor)
		if err != nil {
			result = append(result, approvalFailure(item.ID, 0, err))
		} else {
			result = append(result, ApprovalOutcome{ID: row.ID, RegistrationID: row.RegistrationID, Status: row.Status, CertificateID: row.CertificateID})
		}
	}
	return result, nil
}
func (s Issuance) decideApproval(ctx context.Context, item ApprovalDecisionItem, input ApprovalDecisionInput, actor int32) (dbgen.CertificateApproval, error) {
	var empty dbgen.CertificateApproval
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return empty, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	row, err := q.CertificateApprovalByID(ctx, item.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return empty, domain.Fail(404, "APPROVAL_NOT_FOUND")
	}
	if err != nil {
		return empty, err
	}
	if (input.Action == "cancel" && row.RequestedBy != actor) || (input.Action != "cancel" && row.SignerID != actor) {
		return empty, domain.Fail(403, "APPROVAL_SIGNER_REQUIRED")
	}
	finalStatus := map[string]string{"approve": "approved", "reject": "rejected", "cancel": "cancelled"}[input.Action]
	if row.ContentHash != item.ContentHash {
		return empty, domain.Fail(409, "CERTIFICATE_CONTEXT_CHANGED")
	}
	if row.Status == finalStatus {
		return row, tx.Commit(ctx)
	}
	if row.Status != "pending" {
		return empty, domain.Fail(409, "APPROVAL_ALREADY_DECIDED")
	}
	// Match issuance lock ordering: registration, activity, template, approval.
	var current source
	if input.Action == "approve" {
		current, err = s.load(ctx, q, float64(row.RegistrationID), true)
	} else {
		_, err = q.LockCertificateRegistrationByIdentifier(ctx, strconv.Itoa(int(row.RegistrationID)))
	}
	if err != nil {
		return empty, err
	}
	row, err = q.LockCertificateApproval(ctx, item.ID)
	if err != nil {
		return empty, err
	}
	if row.ContentHash != item.ContentHash {
		return empty, domain.Fail(409, "CERTIFICATE_CONTEXT_CHANGED")
	}
	if row.Status == finalStatus {
		return row, tx.Commit(ctx)
	}
	if row.Status != "pending" {
		return empty, domain.Fail(409, "APPROVAL_ALREADY_DECIDED")
	}
	now := pgtype.Timestamptz{Time: time.Now().Truncate(time.Millisecond), Valid: true}
	var certificateID *int32
	if input.Action == "approve" {
		signer, err := q.LockAdmin(ctx, actor)
		if err != nil {
			return empty, err
		}
		if !auth.ForRole(signer.RoleCode, signer.IsActive).Allows("certificate.approve") || signer.DisplayName == nil || *signer.DisplayName != row.SignerName {
			return empty, domain.Fail(403, "APPROVAL_SIGNER_REQUIRED")
		}
		hash, err := ApprovalHash(current.Data, row.SignerID, row.SignerName, row.SignerTitle)
		if err != nil {
			return empty, err
		}
		var snapshot Response
		if err = json.Unmarshal(row.Snapshot, &snapshot); err != nil {
			return empty, err
		}
		snapshotHash, err := ApprovalHash(snapshot, row.SignerID, row.SignerName, row.SignerTitle)
		if err != nil {
			return empty, err
		}
		if hash != row.ContentHash || snapshotHash != row.ContentHash {
			return empty, domain.Fail(409, "CERTIFICATE_CONTEXT_CHANGED")
		}
		code, err := GenerateCode(current.Activity.ID, now.Time, rand.Reader)
		if err != nil {
			return empty, err
		}
		templateJSON, _ := json.Marshal(snapshot.Template)
		participantJSON, _ := json.Marshal(snapshot.Participant)
		activityJSON, _ := json.Marshal(snapshot.Activity)
		id, err := q.InsertIssuedCertificate(ctx, dbgen.InsertIssuedCertificateParams{Code: code, RegistrationID: row.RegistrationID, ActivityID: row.ActivityID, UserID: snapshot.Participant.UserID, TemplateID: snapshot.Template.ID, TemplateSnapshot: templateJSON, ParticipantSnapshot: participantJSON, ActivitySnapshot: activityJSON, TemplateVersion: snapshot.Template.Version, IssuedBy: &row.RequestedBy, IssuedAt: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return empty, domain.Fail(409, "CERTIFICATE_ALREADY_ISSUED")
		}
		if err != nil {
			return empty, err
		}
		certificateID = &id
		evidence, _ := json.Marshal(ApprovalEvidence{RequestID: row.ID, SignerID: actor, SignerName: row.SignerName, SignerTitle: row.SignerTitle, ApprovedAt: ISO(now.Time, s.Location), ContentHash: hash})
		if err = q.SetCertificateApprovalSnapshot(ctx, dbgen.SetCertificateApprovalSnapshotParams{ID: id, ApprovalSnapshot: evidence}); err != nil {
			return empty, err
		}
	}
	row, err = q.DecideCertificateApproval(ctx, dbgen.DecideCertificateApprovalParams{ID: row.ID, Status: finalStatus, DecidedBy: &actor, DecidedAt: now, Reason: nonempty(strings.TrimSpace(input.Reason)), CertificateID: certificateID})
	if err != nil {
		return empty, err
	}
	return row, tx.Commit(ctx)
}
