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
	ActivityID      int32 `json:"activity_id"`
	TemplateID      int32 `json:"template_id"`
	TemplateVersion int32 `json:"template_version"`
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
type source struct {
	Registration, Activity, Template database.Object
	Data                             Response
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
	return t.In(location).Format("2006-01-02T15:04:05.000Z07:00")
}
func stringPointer(value database.Object, key string) *string {
	if !value.Has(key) || value.Null(key) {
		return nil
	}
	v := value.String(key)
	return &v
}
func ActivityTemplateID(activity database.Object) int32 {
	if !activity.Null("certificate_template_id") && activity.Has("certificate_template_id") {
		return activity.ID("certificate_template_id")
	}
	return rawObject(activity["additional_config"]).ID("certificate_template_id")
}
func (s Issuance) activityData(activity database.Object) ActivityData {
	data := ActivityData{ID: activity.ID("id"), Name: activity.String("name")}
	if raw := activity.String("activity_start"); raw != "" {
		if date, err := time.ParseInLocation("2006-01-02", raw, s.Location); err == nil {
			value := ISO(date, s.Location)
			data.Start = &value
		}
	}
	return data
}
func templateSnapshot(template database.Object) TemplateSnapshot {
	return TemplateSnapshot{ID: template.ID("id"), Name: template.String("name"), Version: template.ID("version"), Background: stringPointer(template, "background_image"), Data: template["template_data"]}
}

var indonesianMonths = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

func (s Issuance) participant(ctx context.Context, q database.JSONQueries, registration, activity database.Object) (Participant, error) {
	data := Participant{RegistrationID: registration.ID("id"), ActivityName: activity.String("name")}
	if date, err := time.Parse("2006-01-02", activity.String("activity_start")); err == nil {
		data.ActivityDate = fmt.Sprintf("%02d %s %04d", date.Day(), indonesianMonths[int(date.Month())-1], date.Year())
	}
	if registration.Null("user_id") {
		guest := rawObject(registration["guest_data"])
		data.Name = strings.TrimSpace(guest.String("name"))
		if data.Name == "" {
			data.Name = "Unknown"
		}
		data.Email = strings.TrimSpace(guest.String("email"))
		data.Gender = strings.TrimSpace(guest.String("gender"))
		data.University = strings.TrimSpace(guest.String("university"))
		if data.University == "" {
			var id int32
			value := guest["university_id"]
			if len(value) > 0 {
				if json.Unmarshal(value, &id) != nil {
					str := guest.String("university_id")
					if positiveIDPattern.MatchString(str) {
						_, _ = fmt.Sscan(str, &id)
					}
				}
			}
			if id > 0 {
				university, err := q.One(ctx, "SELECT name FROM universities WHERE id=$1", id)
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return data, err
				}
				data.University = university.String("name")
			}
		}
	} else {
		id := registration.ID("user_id")
		data.UserID = &id
		user, err := q.One(ctx, "SELECT * FROM public_users WHERE id=$1", id)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return data, err
		}
		profile, err := q.One(ctx, "SELECT p.*,university.name AS university_name FROM profiles p LEFT JOIN universities university ON university.id=p.university_id WHERE p.user_id=$1 ORDER BY p.id LIMIT 1", id)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return data, err
		}
		data.Email = user.String("email")
		data.Name = profile.String("name")
		if data.Name == "" {
			data.Name = data.Email
		}
		if data.Name == "" {
			data.Name = "Unknown"
		}
		data.Gender = profile.String("gender")
		data.University = profile.String("university_name")
	}
	return data, nil
}
func (s Issuance) load(ctx context.Context, q database.JSONQueries, id int32, lock bool) (source, error) {
	suffix := ""
	if lock {
		suffix = " FOR UPDATE"
	}
	registration, err := q.One(ctx, "SELECT * FROM activity_registrations WHERE id=$1"+suffix, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return source{}, Error("REGISTRATION_NOT_FOUND")
	}
	if err != nil {
		return source{}, err
	}
	if registration.String("status") != EligibleStatus {
		return source{}, Error("REGISTRATION_NOT_ELIGIBLE")
	}
	activity, err := q.One(ctx, "SELECT * FROM activities WHERE id=$1"+suffix, registration.ID("activity_id"))
	if errors.Is(err, pgx.ErrNoRows) {
		return source{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return source{}, err
	}
	id = ActivityTemplateID(activity)
	if id == 0 {
		return source{}, Error("NO_CERTIFICATE_TEMPLATE")
	}
	template, err := q.One(ctx, "SELECT * FROM certificate_templates WHERE id=$1"+suffix, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return source{}, Error("CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return source{}, err
	}
	if template.String("lifecycle_status") != "published" {
		return source{}, Error("CERTIFICATE_TEMPLATE_NOT_PUBLISHED")
	}
	readiness := CheckReadiness(template)
	if !readiness.Ready {
		return source{}, Error("CERTIFICATE_TEMPLATE_NOT_READY", readiness.Errors)
	}
	participant, err := s.participant(ctx, q, registration, activity)
	if err != nil {
		return source{}, err
	}
	return source{registration, activity, template, Response{Activity: s.activityData(activity), Template: templateSnapshot(template), Participant: participant}}, nil
}
func (s Issuance) Preview(ctx context.Context, id int32) (Response, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return Response{}, err
	}
	defer tx.Rollback(ctx)
	source, err := s.load(ctx, database.JSONQueries{DB: tx}, id, false)
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
func (s Issuance) checkExpected(ctx context.Context, q database.JSONQueries, id int32, expected Expectation) error {
	registration, err := q.One(ctx, "SELECT * FROM activity_registrations WHERE id=$1 FOR UPDATE", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Error("REGISTRATION_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	activity, err := q.One(ctx, "SELECT * FROM activities WHERE id=$1 FOR UPDATE", registration.ID("activity_id"))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	template, err := q.One(ctx, "SELECT * FROM certificate_templates WHERE id=$1 FOR UPDATE", expected.TemplateID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if activity == nil || activity.ID("id") != expected.ActivityID || ActivityTemplateID(activity) != expected.TemplateID || template == nil || template.ID("version") != expected.TemplateVersion || template.String("lifecycle_status") != "published" || !CheckReadiness(template).Ready {
		return Error("CERTIFICATE_CONTEXT_CHANGED")
	}
	return nil
}
func (s Issuance) Issue(ctx context.Context, id int32, actor *int32, requestID string, expected *Expectation) (IssueResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return IssueResult{}, err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	typed := dbgen.New(tx)
	if expected != nil {
		if err = s.checkExpected(ctx, q, id, *expected); err != nil {
			return IssueResult{}, err
		}
	}
	issued, err := typed.IssuedByRegistration(ctx, id)
	created := false
	if errors.Is(err, pgx.ErrNoRows) {
		source, err := s.load(ctx, q, id, true)
		if err != nil {
			return IssueResult{}, err
		}
		now := time.Now().Truncate(time.Millisecond)
		code, err := GenerateCode(source.Activity.ID("id"), now, rand.Reader)
		if err != nil {
			return IssueResult{}, err
		}
		templateJSON, _ := json.Marshal(source.Data.Template)
		participantJSON, _ := json.Marshal(source.Data.Participant)
		activityJSON, _ := json.Marshal(source.Data.Activity)
		_, err = typed.InsertIssuedCertificate(ctx, dbgen.InsertIssuedCertificateParams{Code: code, RegistrationID: id, ActivityID: source.Activity.ID("id"), UserID: source.Data.Participant.UserID, TemplateID: source.Template.ID("id"), TemplateSnapshot: templateJSON, ParticipantSnapshot: participantJSON, ActivitySnapshot: activityJSON, TemplateVersion: source.Template.ID("version"), IssuedBy: actor, IssuedAt: pgtype.Timestamptz{Time: now, Valid: true}})
		created = err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return IssueResult{}, err
		}
		issued, err = typed.IssuedByRegistration(ctx, id)
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
