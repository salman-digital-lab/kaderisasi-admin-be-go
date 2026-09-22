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
	profileFields := export.ClubProfileFields(form.FormSchema)
	headers := append([]string{"No"}, export.ClubProfileHeaders(profileFields)...)
	headers = append(headers, "Status", "Tanggal Pendaftaran")
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
		registeredAt := ""
		if row.ClubRegistration.CreatedAt.Valid {
			registeredAt = row.ClubRegistration.CreatedAt.Time.In(domain.Jakarta()).Format("2006-01-02 15:04:05")
		}
		locations := export.RegistrationLocations{Province: row.Province, City: row.City, OriginProvince: row.OriginProvince, OriginCity: row.OriginCity, University: row.University}
		values := append([]interface{}{i + 1}, export.ClubProfileRow(profileFields, user.Profile.Value, user.Email, locations)...)
		values = append(values, stringValue(row.ClubRegistration.Status), registeredAt)
		var submitted map[string]json.RawMessage
		_ = json.Unmarshal(row.ClubRegistration.AdditionalData, &submitted)
		for _, question := range questions {
			values = append(values, question.Answer(submitted[question.Key]))
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
