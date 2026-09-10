package certificate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"strconv"
	"strings"
	"time"
)

type source struct {
	Registration dbgen.ActivityRegistration
	Activity     dbgen.Activity
	Template     dbgen.CertificateTemplate
	Data         Response
}

func activityTemplateValue(activity dbgen.Activity) json.RawMessage {
	if activity.CertificateTemplateID != nil {
		return json.RawMessage(strconv.FormatInt(int64(*activity.CertificateTemplateID), 10))
	}
	var config struct {
		TemplateID json.RawMessage `json:"certificate_template_id"`
	}
	_ = json.Unmarshal(activity.AdditionalConfig, &config)
	return config.TemplateID
}
func activityTemplateIdentifier(activity dbgen.Activity) *string {
	raw := activityTemplateValue(activity)
	if !database.JSONTruthy(raw) {
		return nil
	}
	value := database.JSONParameter(raw)
	return &value
}
func (s Issuance) activityData(activity dbgen.Activity) ActivityData {
	data := ActivityData{ID: activity.ID, Name: activity.Name}
	if activity.ActivityStart.Valid {
		date := activity.ActivityStart.Time
		value := ISO(time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, s.Location), s.Location)
		data.Start = &value
	}
	return data
}
func templateSnapshot(template dbgen.CertificateTemplate) TemplateSnapshot {
	return TemplateSnapshot{ID: template.ID, Name: template.Name, Version: template.Version, Background: template.BackgroundImage, Data: template.TemplateData}
}

var indonesianMonths = []string{"Januari", "Februari", "Maret", "April", "Mei", "Juni", "Juli", "Agustus", "September", "Oktober", "November", "Desember"}

type participantGuest struct {
	Name         json.RawMessage `json:"name"`
	Email        json.RawMessage `json:"email"`
	Gender       json.RawMessage `json:"gender"`
	University   json.RawMessage `json:"university"`
	UniversityID json.RawMessage `json:"university_id"`
}

func guestText(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return strings.TrimSpace(value)
}
func (s Issuance) participant(ctx context.Context, q *dbgen.Queries, registration dbgen.ActivityRegistration, activity dbgen.Activity) (Participant, error) {
	data := Participant{RegistrationID: registration.ID, UserID: registration.UserID, ActivityName: activity.Name}
	if activity.ActivityStart.Valid {
		date := activity.ActivityStart.Time
		data.ActivityDate = fmt.Sprintf("%02d %s %04d", date.Day(), indonesianMonths[int(date.Month())-1], date.Year())
	}
	if registration.UserID == nil {
		var guest participantGuest
		_ = json.Unmarshal(registration.GuestData, &guest)
		data.Name = guestText(guest.Name)
		if data.Name == "" {
			data.Name = "Unknown"
		}
		data.Email = guestText(guest.Email)
		data.Gender = guestText(guest.Gender)
		data.University = guestText(guest.University)
		if data.University == "" {
			var number float64
			raw := guest.UniversityID
			valid := len(raw) > 0 && string(raw) != "null" && json.Unmarshal(raw, &number) == nil
			if !valid {
				var value string
				if json.Unmarshal(raw, &value) == nil && positiveIDPattern.MatchString(value) {
					number, _ = strconv.ParseFloat(database.NumberIdentifier(value), 64)
					valid = true
				}
			}
			if valid && number != 0 {
				name, err := q.CertificateGuestUniversity(ctx, database.JSNumber(number))
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return data, err
				}
				data.University = name
			}
		}
	} else {
		member, err := q.CertificateParticipantMember(ctx, *registration.UserID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return data, err
		}
		data.Email = member.Email
		data.Name = member.Name
		data.Gender = member.Gender
		data.University = member.University
		if data.Name == "" {
			data.Name = data.Email
		}
		if data.Name == "" {
			data.Name = "Unknown"
		}
	}
	return data, nil
}
func registrationActivityID(row dbgen.ActivityRegistration) string {
	if row.ActivityID == nil {
		return "0"
	}
	return strconv.FormatInt(int64(*row.ActivityID), 10)
}
func (s Issuance) load(ctx context.Context, q *dbgen.Queries, id float64, lock bool) (source, error) {
	findRegistration := q.CertificateRegistrationByIdentifier
	findActivity := q.CertificateActivityByIdentifier
	findTemplate := q.CertificateTemplateByIdentifier
	if lock {
		findRegistration = q.LockCertificateRegistrationByIdentifier
		findActivity = q.LockCertificateActivityByIdentifier
		findTemplate = q.LockCertificateTemplate
	}
	registration, err := findRegistration(ctx, database.JSNumber(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return source{}, Error("REGISTRATION_NOT_FOUND")
	}
	if err != nil {
		return source{}, err
	}
	if registration.Status == nil || *registration.Status != EligibleStatus {
		return source{}, Error("REGISTRATION_NOT_ELIGIBLE")
	}
	activity, err := findActivity(ctx, registrationActivityID(registration))
	if errors.Is(err, pgx.ErrNoRows) {
		return source{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return source{}, err
	}
	templateID := activityTemplateIdentifier(activity)
	if templateID == nil {
		return source{}, Error("NO_CERTIFICATE_TEMPLATE")
	}
	template, err := findTemplate(ctx, *templateID)
	if errors.Is(err, pgx.ErrNoRows) {
		return source{}, Error("CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return source{}, err
	}
	if template.LifecycleStatus != "published" {
		return source{}, Error("CERTIFICATE_TEMPLATE_NOT_PUBLISHED")
	}
	readiness := CheckReadinessValues(template.ID, template.Name, template.TemplateData)
	if !readiness.Ready {
		return source{}, Error("CERTIFICATE_TEMPLATE_NOT_READY", readiness.Errors)
	}
	participant, err := s.participant(ctx, q, registration, activity)
	if err != nil {
		return source{}, err
	}
	return source{Registration: registration, Activity: activity, Template: template, Data: Response{Activity: s.activityData(activity), Template: templateSnapshot(template), Participant: participant}}, nil
}
func (s Issuance) checkExpected(ctx context.Context, q *dbgen.Queries, id float64, expected Expectation) error {
	registration, err := q.LockCertificateRegistrationByIdentifier(ctx, database.JSNumber(id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Error("REGISTRATION_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	activity, activityErr := q.LockCertificateActivityByIdentifier(ctx, registrationActivityID(registration))
	if activityErr != nil && !errors.Is(activityErr, pgx.ErrNoRows) {
		return activityErr
	}
	template, templateErr := q.LockCertificateTemplate(ctx, database.JSNumber(expected.TemplateID))
	if templateErr != nil && !errors.Is(templateErr, pgx.ErrNoRows) {
		return templateErr
	}
	var assigned float64
	assignmentErr := json.Unmarshal(activityTemplateValue(activity), &assigned)
	if activityErr != nil || float64(activity.ID) != expected.ActivityID || assignmentErr != nil || assigned != expected.TemplateID || templateErr != nil || float64(template.Version) != expected.TemplateVersion || template.LifecycleStatus != "published" || !CheckReadinessValues(template.ID, template.Name, template.TemplateData).Ready {
		return Error("CERTIFICATE_CONTEXT_CHANGED")
	}
	return nil
}
