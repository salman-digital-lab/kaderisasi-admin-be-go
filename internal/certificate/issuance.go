package certificate

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"log/slog"
	"regexp"
	"strings"
	"time"
)

const EligibleStatus = "LULUS KEGIATAN"

var positiveIDPattern = regexp.MustCompile(`^[0-9]+$`)

type Participant struct {
	RegistrationID int32  `json:"registration_id"`
	UserID         *int32 `json:"user_id"`
	Name           string `json:"name"`
	Email          string `json:"email"`
	University     string `json:"university"`
	Gender         string `json:"gender"`
	ActivityName   string `json:"activity_name"`
	ActivityDate   string `json:"activity_date"`
}
type ActivityData struct {
	ID    int32   `json:"id"`
	Name  string  `json:"name"`
	Start *string `json:"activity_start"`
}
type TemplateSnapshot struct {
	ID         int32           `json:"id"`
	Name       string          `json:"name"`
	Version    int32           `json:"version"`
	Background *string         `json:"background_image"`
	Data       json.RawMessage `json:"template_data"`
}
type IssuedData struct {
	ID              int32   `json:"id"`
	Code            string  `json:"certificate_code"`
	RegistrationID  int32   `json:"registration_id"`
	ActivityID      int32   `json:"activity_id"`
	TemplateID      int32   `json:"template_id"`
	TemplateVersion int32   `json:"template_version"`
	IssuedAt        string  `json:"issued_at"`
	IssuedBy        *int32  `json:"issued_by"`
	RevokedAt       *string `json:"revoked_at"`
	RevokedReason   *string `json:"revoked_reason"`
	RevokedBy       *int32  `json:"revoked_by"`
}
type Response struct {
	Activity    ActivityData     `json:"activity"`
	Template    TemplateSnapshot `json:"template"`
	Participant Participant      `json:"participant"`
	Certificate *IssuedData      `json:"certificate,omitempty"`
}
type Expectation struct {
	ActivityID      float64 `json:"activity_id"`
	TemplateID      float64 `json:"template_id"`
	TemplateVersion float64 `json:"template_version"`
}
type IssueResult struct {
	Data    Response
	Issued  dbgen.IssuedCertificate
	Created bool
}
type Issuance struct {
	Pool     *pgxpool.Pool
	Location *time.Location
	Logger   *slog.Logger
}

func Error(code string, details ...[]string) error {
	status := 400
	switch code {
	case "REGISTRATION_NOT_FOUND", "ACTIVITY_NOT_FOUND", "CERTIFICATE_TEMPLATE_NOT_FOUND", "CERTIFICATE_NOT_FOUND":
		status = 404
	case "CERTIFICATE_CONTEXT_CHANGED", "REGISTRATION_NOT_ELIGIBLE", "CERTIFICATE_ALREADY_REVOKED":
		status = 409
	case "NO_CERTIFICATE_TEMPLATE", "CERTIFICATE_TEMPLATE_NOT_PUBLISHED", "CERTIFICATE_TEMPLATE_NOT_READY":
		status = 422
	}
	if len(details) > 0 {
		return domain.Details(status, code, map[string]interface{}{"errors": details[0]})
	}
	return domain.Fail(status, code)
}
func GenerateCode(activityID int32, now time.Time, entropy io.Reader) (string, error) {
	if activityID <= 0 {
		return "", errors.New("INVALID_ACTIVITY_ID")
	}
	raw := make([]byte, 16)
	if _, err := io.ReadFull(entropy, raw); err != nil {
		return "", err
	}
	return fmt.Sprintf("CERT-%d-%d-%s", now.UTC().Year(), activityID, strings.ToUpper(hex.EncodeToString(raw))), nil
}
func ISO(t time.Time, location *time.Location) string {
	if location == nil {
		location = time.Local
	}
	return t.In(location).Format("2006-01-02T15:04:05.000-07:00")
}
func (s Issuance) Preview(ctx context.Context, id float64) (Response, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Response{}, err
	}
	defer tx.Rollback(ctx)
	source, err := s.load(ctx, dbgen.New(tx), id, false)
	if err != nil {
		return Response{}, err
	}
	return source.Data, tx.Commit(ctx)
}
func (s Issuance) IssuedResponse(row dbgen.IssuedCertificate) (Response, error) {
	var result Response
	if err := json.Unmarshal(row.ParticipantSnapshot, &result.Participant); err != nil {
		return result, err
	}
	if err := json.Unmarshal(row.TemplateSnapshot, &result.Template); err != nil {
		return result, err
	}
	if len(row.ActivitySnapshot) > 0 && string(row.ActivitySnapshot) != "null" {
		if err := json.Unmarshal(row.ActivitySnapshot, &result.Activity); err != nil {
			return result, err
		}
	} else {
		result.Activity = ActivityData{ID: row.ActivityID, Name: result.Participant.ActivityName}
	}
	issued := &IssuedData{ID: row.ID, Code: row.CertificateCode, RegistrationID: row.RegistrationID, ActivityID: row.ActivityID, TemplateID: row.TemplateID, TemplateVersion: row.TemplateVersion, IssuedAt: ISO(row.IssuedAt.Time, s.Location), IssuedBy: row.IssuedBy, RevokedBy: row.RevokedBy, RevokedReason: row.RevokedReason}
	if row.RevokedAt.Valid {
		value := ISO(row.RevokedAt.Time, s.Location)
		issued.RevokedAt = &value
	}
	result.Certificate = issued
	return result, nil
}
func (s Issuance) Issue(ctx context.Context, id float64, actor *int32, requestID string, expected *Expectation) (IssueResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return IssueResult{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	typed := dbgen.New(tx)
	if expected != nil {
		if err = s.checkExpected(ctx, q, id, *expected); err != nil {
			return IssueResult{}, err
		}
	}
	issued, err := typed.IssuedCertificateByRegistrationIdentifier(ctx, database.JSNumber(id))
	created := false
	if errors.Is(err, pgx.ErrNoRows) {
		source, err := s.load(ctx, q, id, true)
		if err != nil {
			return IssueResult{}, err
		}
		now := time.Now().Truncate(time.Millisecond)
		code, err := GenerateCode(source.Activity.ID, now, rand.Reader)
		if err != nil {
			return IssueResult{}, err
		}
		templateJSON, _ := json.Marshal(source.Data.Template)
		participantJSON, _ := json.Marshal(source.Data.Participant)
		activityJSON, _ := json.Marshal(source.Data.Activity)
		_, err = typed.InsertIssuedCertificate(ctx, dbgen.InsertIssuedCertificateParams{Code: code, RegistrationID: source.Registration.ID, ActivityID: source.Activity.ID, UserID: source.Data.Participant.UserID, TemplateID: source.Template.ID, TemplateSnapshot: templateJSON, ParticipantSnapshot: participantJSON, ActivitySnapshot: activityJSON, TemplateVersion: source.Template.Version, IssuedBy: actor, IssuedAt: pgtype.Timestamptz{Time: now, Valid: true}})
		created = err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return IssueResult{}, err
		}
		issued, err = typed.IssuedCertificateByRegistrationIdentifier(ctx, database.JSNumber(id))
		if err != nil {
			return IssueResult{}, err
		}
	} else if err != nil {
		return IssueResult{}, err
	}
	data, err := s.IssuedResponse(issued)
	if err != nil {
		return IssueResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return IssueResult{}, err
	}
	if s.Logger != nil {
		event := "certificate_issue_reused"
		if created {
			event = "certificate_issued"
		}
		s.Logger.Info(event, "event", event, "request_id", requestID, "actor_admin_id", actor, "registration_id", id, "certificate_id", issued.ID, "certificate_code", issued.CertificateCode)
	}
	return IssueResult{Data: data, Issued: issued, Created: created}, nil
}
