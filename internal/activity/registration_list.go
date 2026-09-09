package activity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
)

var registrationSortColumns = map[string]string{"created_at": "activity_registrations.created_at", "name": "profiles.name", "email": "public_users.email", "status": "activity_registrations.status", "level": "profiles.level", "university_id": "profiles.university_id", "province_id": "profiles.province_id", "intake_year": "profiles.intake_year", "major": "profiles.major", "whatsapp": "profiles.whatsapp"}

func registrationProfileFields(raw []byte) ([]string, error) {
	var config struct {
		Mandatory json.RawMessage `json:"mandatory_profile_data"`
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, errors.New("Cannot read properties of null (reading 'mandatory_profile_data')")
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	if len(config.Mandatory) == 0 {
		return nil, errors.New("Cannot read properties of undefined (reading 'map')")
	}
	if string(config.Mandatory) == "null" {
		return nil, errors.New("Cannot read properties of null (reading 'map')")
	}
	var fields []struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(config.Mandatory, &fields); err != nil {
		return nil, errors.New("mandatoryData.map is not a function")
	}
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		result = append(result, field.Name)
	}
	return result, nil
}
func (s Service) ListRegistrations(ctx context.Context, identifier string, filters RegistrationFilters) (RegistrationPage, error) {
	q := dbgen.New(s.Pool)
	activity, err := q.RegistrationActivityByIdentifier(ctx, identifier)
	if err != nil {
		return RegistrationPage{}, database.LegacyQueryError(err, `select * from "activities" where "id" = $1 limit $2`)
	}
	fields, err := registrationProfileFields(activity.AdditionalConfig)
	if err != nil {
		return RegistrationPage{}, err
	}
	if registrationSortColumns[filters.SortBy] == "" {
		filters.SortBy = "created_at"
	}
	known, err := profileProjection(nil)
	if err != nil {
		return RegistrationPage{}, err
	}
	for _, field := range fields {
		if _, exists := known[field]; exists || field == "*" {
			continue
		}
		// A saved configuration can reference a removed/unknown column. Execute
		// that invalid projection to preserve PostgreSQL's real error response.
		rows, err := s.Pool.Query(ctx, "SELECT "+pgx.Identifier{"profiles", field}.Sanitize()+" FROM profiles LIMIT 0")
		if err == nil {
			rows.Close()
			return RegistrationPage{}, fmt.Errorf("Unsupported registration profile column %q", field)
		}
		return RegistrationPage{}, database.LegacyQueryError(err, registrationListStatement(filters, fields, true))
	}
	count, err := q.CountRegistrationsFiltered(ctx, dbgen.CountRegistrationsFilteredParams{ActivityID: activity.ID, Search: filters.Search, Status: filters.Status, UniversityID: filters.UniversityID, ProvinceID: filters.ProvinceID, IntakeYear: filters.IntakeYear})
	if err != nil {
		return RegistrationPage{}, database.LegacyQueryError(err, registrationListStatement(filters, fields, true))
	}
	result := RegistrationPage{Meta: database.Meta(count, filters.Page, filters.Size).Raw(), Data: []RegistrationSummary{}}
	if count == 0 {
		return result, nil
	}
	limit, offset, err := database.SQLPage(filters.Page, filters.Size)
	if err != nil {
		return result, err
	}
	rows, err := q.ListRegistrationsFiltered(ctx, dbgen.ListRegistrationsFilteredParams{ActivityID: activity.ID, Search: filters.Search, Status: filters.Status, UniversityID: filters.UniversityID, ProvinceID: filters.ProvinceID, IntakeYear: filters.IntakeYear, SortBy: filters.SortBy, Ascending: filters.Ascending, PageSize: limit, PageOffset: offset})
	if err != nil {
		return result, database.LegacyQueryError(err, registrationListStatement(filters, fields, false))
	}
	for _, row := range rows {
		view, err := registrationSummary(row, fields)
		if err != nil {
			return result, err
		}
		result.Data = append(result.Data, view)
	}
	return result, nil
}
