package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/export"
	"kaderisasi/admin/internal/member"
)

func (s Service) ExportRegistrations(ctx context.Context, identifier string) (export.Document, error) {
	q := dbgen.New(s.Pool)
	activity, err := q.RegistrationActivityByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return export.Document{}, err
		}
		return export.Document{}, fmt.Errorf("error: %s", database.LegacyQueryError(err, `select * from "activities" where "id" = $1 limit $2`))
	}
	rows, err := q.RegistrationExportRows(ctx, &activity.ID)
	if err != nil {
		return export.Document{}, err
	}
	ids := make([]int32, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	relations, err := q.RegistrationExportRelations(ctx, ids)
	if err != nil {
		return export.Document{}, err
	}
	byID := make(map[int32]dbgen.RegistrationExportRelationsRow, len(relations))
	for _, relation := range relations {
		byID[relation.ID] = relation
	}
	registrations := make([]export.Registration, 0, len(rows))
	for _, row := range rows {
		relation := byID[row.ID]
		record := export.Registration{ID: row.ID, UserID: row.UserID, Email: relation.Email, Answers: row.QuestionnaireAnswer}
		if len(record.Answers) == 0 {
			record.Answers = json.RawMessage(`null`)
		}
		for _, item := range []struct {
			raw    []byte
			target interface{}
		}{{row.GuestData, &record.Guest}, {relation.Locations, &record.Locations}} {
			if len(item.raw) > 0 {
				if err = json.Unmarshal(item.raw, item.target); err != nil {
					return export.Document{}, err
				}
			}
		}
		var profile *registrationProfile
		if len(relation.Profile) > 0 {
			if err = json.Unmarshal(relation.Profile, &profile); err != nil {
				return export.Document{}, err
			}
		}
		if profile != nil {
			profile.Profile.Badges = profile.Badges
			profile.Profile.EducationHistory = profile.EducationHistory
			profile.Profile.WorkHistory = profile.WorkHistory
			profile.Profile.ExtraData = profile.ExtraData
			view := member.ProfileView(profile.Profile)
			record.Profile = &view
		}
		registrations = append(registrations, record)
	}
	var questions []export.Question
	form, err := q.RegistrationExportForm(ctx, &activity.ID)
	if err == nil {
		questions, err = export.RegistrationQuestions(form, true)
	} else if errors.Is(err, pgx.ErrNoRows) {
		questions, err = export.RegistrationQuestions(activity.AdditionalConfig, false)
	}
	if err != nil {
		return export.Document{}, err
	}
	if err = registrationExportLocations(ctx, q, registrations); err != nil {
		return export.Document{}, err
	}
	headers := append([]string{}, export.RegistrationHeaders...)
	for _, question := range questions {
		headers = append(headers, question.Label)
	}
	cells := make([][]interface{}, len(registrations))
	badge := ""
	if activity.Badge != nil {
		badge = *activity.Badge
	}
	for i, record := range registrations {
		cells[i], err = export.RegistrationRow(i+1, record, questions, badge)
		if err != nil {
			return export.Document{}, err
		}
	}
	body, err := export.Workbook("Registrations", headers, cells)
	return export.Document{Filename: export.Filename(activity.Name), Body: body}, err
}
