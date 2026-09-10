package club

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/export"
	"kaderisasi/admin/internal/member"
	"strconv"
	"strings"
)

type RegistrationExport struct {
	Filename string
	Body     []byte
}

func (s Service) ExportRegistrations(ctx context.Context, id string) (RegistrationExport, error) {
	club, err := s.registrationClub(ctx, id)
	if err != nil {
		return RegistrationExport{}, err
	}
	q := dbgen.New(s.Pool)
	rows, err := q.ClubRegistrationExport(ctx, club.ID)
	if err != nil {
		return RegistrationExport{}, err
	}
	form, err := q.ActiveFormByFeature(ctx, dbgen.ActiveFormByFeatureParams{FeatureType: "club_registration", Identifier: id})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return RegistrationExport{}, err
	}
	answers := make([][]byte, 0, len(rows))
	for _, row := range rows {
		answers = append(answers, row.ClubRegistration.AdditionalData)
	}
	questions := export.ClubQuestions(form.FormSchema, answers)
	headers := []string{"No", "Nama Lengkap", "Email", "Whatsapp", "Nomor Identitas", "Provinsi", "Universitas", "Jurusan", "Tahun Masuk", "Jenjang", "Status", "Tanggal Pendaftaran"}
	for _, question := range questions {
		headers = append(headers, question.Label)
	}
	data := make([][]interface{}, 0, len(rows))
	for i, row := range rows {
		user, err := member.UserFromRelation(row.Member, row.Profile, true)
		if err != nil {
			return RegistrationExport{}, err
		}
		if user == nil {
			return RegistrationExport{}, errors.New("Cannot read properties of null (reading 'profile')")
		}
		name, whatsapp, personal, major, level := "", "", "", "", ""
		var intake interface{} = ""
		if profile := user.Profile.Value; profile != nil {
			name = profile.Name
			whatsapp = stringValue(profile.Whatsapp)
			personal = stringValue(profile.PersonalID)
			major = stringValue(profile.Major)
			if profile.IntakeYear != nil && *profile.IntakeYear != 0 {
				intake = *profile.IntakeYear
			}
			number := int32(0)
			if profile.Level != nil {
				number = *profile.Level
			}
			level = strconv.FormatInt(int64(number), 10)
			if label, ok := map[int32]string{0: "JAMAAH", 3: "AKTIVIS", 6: "KADER", 10: "KADER LANJUT"}[number]; ok {
				level = label
			}
		}
		registeredAt := ""
		if row.ClubRegistration.CreatedAt.Valid {
			registeredAt = row.ClubRegistration.CreatedAt.Time.In(domain.Jakarta()).Format("2006-01-02 15:04:05")
		}
		values := []interface{}{i + 1, name, stringValue(user.Email), whatsapp, personal, stringValue(row.Province), stringValue(row.University), major, intake, level, stringValue(row.ClubRegistration.Status), registeredAt}
		var submitted map[string]json.RawMessage
		_ = json.Unmarshal(row.ClubRegistration.AdditionalData, &submitted)
		for _, question := range questions {
			values = append(values, export.ClubAnswer(submitted[question.Key]))
		}
		data = append(data, values)
	}
	body, err := export.Workbook("Registrations", headers, data)
	return RegistrationExport{Filename: strings.TrimSuffix(export.Filename(club.Name), ".xlsx") + "_registrations.xlsx", Body: body}, err
}
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
