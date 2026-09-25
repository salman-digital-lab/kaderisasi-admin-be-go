package certificate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

type Settings struct {
	Version       int    `json:"version"`
	Institution   string `json:"institution"`
	Role          string `json:"role"`
	EventDate     string `json:"event_date"`
	HijriDate     string `json:"hijri_date"`
	DeliveryMode  string `json:"delivery_mode"`
	Venue         string `json:"venue"`
	Organizer     string `json:"organizer"`
	DocumentPlace string `json:"document_place"`
	DocumentDate  string `json:"document_date"`
	IncludeScores bool   `json:"include_scores"`
}

func DefaultSettings(activity dbgen.Activity, hasRubric bool, now time.Time) Settings {
	settings := Settings{Version: 1, Institution: "YAYASAN PEMBINA MASJID (YPM) SALMAN ITB", Role: "PESERTA", Organizer: "Bidang Mahasiswa, Kaderisasi, dan Alumni Salman ITB", DocumentPlace: "Bandung", DocumentDate: fmt.Sprintf("%d %s %d", now.Day(), indonesianMonths[int(now.Month())-1], now.Year()), IncludeScores: hasRubric}
	if activity.ActivityStart.Valid {
		start := activity.ActivityStart.Time
		settings.EventDate = fmt.Sprintf("%d %s %d", start.Day(), indonesianMonths[int(start.Month())-1], start.Year())
		if activity.ActivityEnd.Valid && !activity.ActivityStart.Time.Equal(activity.ActivityEnd.Time) {
			end := activity.ActivityEnd.Time
			settings.EventDate += fmt.Sprintf(" s.d. %d %s %d", end.Day(), indonesianMonths[int(end.Month())-1], end.Year())
		}
	}
	return settings
}

func ParseSettings(activity dbgen.Activity, hasRubric bool, now time.Time) (Settings, error) {
	settings := DefaultSettings(activity, hasRubric, now)
	var config struct {
		Settings json.RawMessage `json:"certificate_settings"`
	}
	if err := json.Unmarshal(activity.AdditionalConfig, &config); err != nil && len(activity.AdditionalConfig) > 0 {
		return settings, err
	}
	if len(config.Settings) > 0 && string(config.Settings) != "null" {
		if err := json.Unmarshal(config.Settings, &settings); err != nil {
			return settings, err
		}
	}
	return settings, nil
}

func HasSavedSettings(activity dbgen.Activity) bool {
	var config struct {
		Settings json.RawMessage `json:"certificate_settings"`
	}
	return json.Unmarshal(activity.AdditionalConfig, &config) == nil && len(config.Settings) > 0 && string(config.Settings) != "null"
}

func ValidSettings(value Settings) bool {
	if value.Version != 1 || strings.TrimSpace(value.Institution) == "" || strings.TrimSpace(value.Role) == "" || strings.TrimSpace(value.Organizer) == "" || strings.TrimSpace(value.DocumentPlace) == "" || strings.TrimSpace(value.DocumentDate) == "" {
		return false
	}
	for _, field := range []string{value.Institution, value.Role, value.EventDate, value.HijriDate, value.DeliveryMode, value.Venue, value.Organizer, value.DocumentPlace, value.DocumentDate} {
		if utf8.RuneCountInString(field) > 500 {
			return false
		}
	}
	return true
}

func (s Issuance) Settings(ctx context.Context, id int32) (Settings, error) {
	q := dbgen.New(s.Pool)
	activity, err := q.CertificateActivityByIdentifier(ctx, strconv.FormatInt(int64(id), 10))
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return Settings{}, err
	}
	hasRubric, err := q.CertificateActivityHasScoringRubric(ctx, id)
	if err != nil {
		return Settings{}, err
	}
	now := time.Now()
	if s.Location != nil {
		now = now.In(s.Location)
	}
	return ParseSettings(activity, hasRubric, now)
}

func (s Issuance) SaveSettings(ctx context.Context, id int32, settings Settings) (Settings, error) {
	if !ValidSettings(settings) {
		return Settings{}, domain.Fail(422, "INVALID_CERTIFICATE_SETTINGS")
	}
	raw, err := json.Marshal(settings)
	if err != nil {
		return Settings{}, err
	}
	row, err := dbgen.New(s.Pool).SetActivityCertificateSettings(ctx, dbgen.SetActivityCertificateSettingsParams{ActivityID: id, Settings: raw})
	if errors.Is(err, pgx.ErrNoRows) {
		return Settings{}, Error("ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return Settings{}, err
	}
	return ParseSettings(row, false, time.Now())
}

func (s Issuance) SaveGroup(ctx context.Context, activityID, registrationID int32, group *string) error {
	if group != nil {
		value := strings.TrimSpace(*group)
		if utf8.RuneCountInString(value) > 100 {
			return domain.Fail(422, "INVALID_CERTIFICATE_GROUP")
		}
		if value == "" {
			group = nil
		} else {
			group = &value
		}
	}
	count, err := dbgen.New(s.Pool).UpdateCertificateGroup(ctx, dbgen.UpdateCertificateGroupParams{ActivityID: activityID, RegistrationID: registrationID, GroupLabel: group})
	if err != nil {
		return err
	}
	if count == 0 {
		return domain.Fail(409, "CERTIFICATE_GROUP_NOT_EDITABLE")
	}
	return nil
}
