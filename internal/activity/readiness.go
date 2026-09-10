package activity

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"html"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/form"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type SetupIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Step    int    `json:"step"`
	Scope   string `json:"scope"`
}
type SetupActions struct {
	CanEdit               bool `json:"can_edit"`
	CanPublish            bool `json:"can_publish"`
	CanManageRegistration bool `json:"can_manage_registration"`
}
type Readiness struct {
	CanPublish          bool         `json:"can_publish"`
	CanOpenRegistration bool         `json:"can_open_registration"`
	Issues              []SetupIssue `json:"issues"`
	Actions             SetupActions `json:"actions"`
}

var markup = regexp.MustCompile(`<[^>]*>`)

func hasDescription(description *string) bool {
	return description != nil && strings.TrimSpace(html.UnescapeString(markup.ReplaceAllString(*description, ""))) != ""
}

func EvaluateReadiness(row dbgen.Activity, activeForm *dbgen.CustomForm, now time.Time) Readiness {
	r := Readiness{CanPublish: true, CanOpenRegistration: true, Issues: []SetupIssue{}}
	add := func(code, message string, step int, scope string) {
		r.Issues = append(r.Issues, SetupIssue{code, message, step, scope})
		if scope == "publication" {
			r.CanPublish = false
		}
		r.CanOpenRegistration = false
	}
	if strings.TrimSpace(row.Name) == "" {
		add("name", "Isi nama kegiatan.", 0, "publication")
	}
	if row.ActivityType == nil || !slices.Contains([]int32{0, 1, 2, 3, 4, 5, 20}, *row.ActivityType) {
		add("type", "Pilih tipe kegiatan.", 0, "publication")
	}
	if row.ActivityCategory == nil || *row.ActivityCategory < 0 || *row.ActivityCategory > 4 {
		add("category", "Pilih kategori kegiatan.", 0, "publication")
	}
	if row.MinimumLevel == nil || !slices.Contains([]int32{0, 3, 6, 10}, *row.MinimumLevel) {
		add("level", "Pilih minimum jenjang peserta.", 0, "publication")
	}
	if !hasDescription(row.Description) {
		add("description", "Tulis deskripsi agar calon peserta memahami kegiatan.", 1, "publication")
	}
	var config struct {
		Images []string `json:"images"`
	}
	posterPresent := false
	if json.Unmarshal(row.AdditionalConfig, &config) == nil {
		for _, image := range config.Images {
			if strings.TrimSpace(image) != "" {
				posterPresent = true
				break
			}
		}
	}
	if !posterPresent {
		add("poster", "Unggah minimal satu poster kegiatan.", 1, "publication")
	}
	if !orderedDates(row.ActivityStart, row.ActivityEnd) {
		add("activity_dates", "Lengkapi kedua tanggal kegiatan dan pastikan tanggal selesai tidak mendahului tanggal mulai.", 0, "publication")
	}
	if !orderedDates(row.RegistrationStart, row.RegistrationEnd) {
		add("registration_dates", "Lengkapi kedua tanggal pendaftaran dan pastikan urutannya benar.", 2, "publication")
	}
	if !row.RegistrationStart.Valid || !row.RegistrationEnd.Valid {
		add("registration_period", "Tentukan periode pendaftaran.", 2, "registration")
	} else {
		today := now.Format("2006-01-02")
		if today < row.RegistrationStart.Time.Format("2006-01-02") {
			add("registration_future", "Pendaftaran belum dimulai. Admin dapat membukanya pada tanggal mulai.", 2, "registration")
		}
		if today > row.RegistrationEnd.Time.Format("2006-01-02") {
			add("registration_expired", "Periode pendaftaran sudah berakhir. Perbarui tanggal sebelum membukanya.", 2, "registration")
		}
	}
	if activeForm == nil || activeForm.IsActive == nil || !*activeForm.IsActive || !form.ValidSchema(activeForm.FormSchema) {
		add("registration_form", "Siapkan dan aktifkan formulir pendaftaran yang valid.", 2, "registration")
	}
	return r
}

func orderedDates(start, end pgtype.Date) bool {
	return start.Valid == end.Valid && (!start.Valid || !end.Time.Before(start.Time))
}
func withActions(r Readiness, permissions auth.Authorization) Readiness {
	r.Actions = SetupActions{permissions.Allows("activities.manage"), permissions.Allows("activities.publish"), permissions.Allows("activities.registration.manage")}
	return r
}
func activeRegistrationForm(ctx context.Context, q *dbgen.Queries, id int32) (*dbgen.CustomForm, error) {
	f, err := q.ActiveFormByFeature(ctx, dbgen.ActiveFormByFeatureParams{FeatureType: "activity_registration", Identifier: strconv.Itoa(int(id))})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}
func (s Service) Readiness(ctx context.Context, id string, permissions auth.Authorization) (Readiness, error) {
	q := dbgen.New(s.Pool)
	row, err := q.ActivityByIdentifier(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Readiness{}, domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return Readiness{}, err
	}
	form, err := activeRegistrationForm(ctx, q, row.ID)
	if err != nil {
		return Readiness{}, err
	}
	location, _ := time.LoadLocation("Asia/Jakarta")
	return withActions(EvaluateReadiness(row, form, time.Now().In(location)), permissions), nil
}

func authorizeTransition(old dbgen.Activity, data Input, permissions auth.Authorization) error {
	published := old.IsPublished != nil && *old.IsPublished
	if data.IsPublished != nil {
		value, err := data.IsPublished.Int64()
		if err != nil || (value != 0 && value != 1) {
			return domain.Fail(422, "INVALID_PUBLICATION_STATUS")
		}
		if (value == 1) != published && !permissions.Allows("activities.publish") {
			return domain.Fail(403, "ACTIVITY_PUBLICATION_FORBIDDEN")
		}
	}
	if data.IsRegistrationOpen != nil && *data.IsRegistrationOpen != old.IsRegistrationOpen && !permissions.Allows("activities.registration.manage") {
		return domain.Fail(403, "ACTIVITY_REGISTRATION_CONTROL_FORBIDDEN")
	}
	return nil
}
