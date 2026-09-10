package activity

import (
	"encoding/json"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/member"
	"time"
)

type RegistrationFilters struct {
	Search, Status, UniversityID, ProvinceID, IntakeYear *string
	SortBy                                               string
	Ascending                                            bool
	Page, Size                                           float64
}
type RegistrationPage struct {
	Meta database.RawPagination `json:"meta"`
	Data []RegistrationSummary  `json:"data"`
}
type RegistrationSummary struct {
	ID               int32                      `json:"id"`
	UserID           *int32                     `json:"user_id"`
	Email            *string                    `json:"email"`
	Name             *string                    `json:"name"`
	Level            *int32                     `json:"level"`
	UniversityID     *int32                     `json:"university_id"`
	ProvinceID       *int32                     `json:"province_id"`
	IntakeYear       *int32                     `json:"intake_year"`
	Major            *string                    `json:"major"`
	Gender           *string                    `json:"gender"`
	Whatsapp         *string                    `json:"whatsapp"`
	Instagram        *string                    `json:"instagram"`
	Line             *string                    `json:"line"`
	PersonalID       *string                    `json:"personal_id"`
	EducationHistory json.RawMessage            `json:"education_history"`
	GuestData        json.RawMessage            `json:"guest_data"`
	Status           *string                    `json:"status"`
	CreatedAt        *string                    `json:"created_at"`
	ProfileFields    map[string]json.RawMessage `json:"-"`
}

// Mandatory profile fields are a genuinely dynamic part of this endpoint. The
// selected columns override the fixed projection, except status and created_at
// which the source selects last. Values come from the typed profile projection.
func (row RegistrationSummary) MarshalJSON() ([]byte, error) {
	type plain RegistrationSummary
	raw, err := json.Marshal(plain(row))
	if err != nil {
		return nil, err
	}
	var result map[string]json.RawMessage
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	for key, value := range row.ProfileFields {
		if key != "status" && key != "created_at" {
			result[key] = value
		}
	}
	result["education_history"] = member.NormalizeEducationHistory(result["education_history"])
	if raw, ok := result["work_history"]; ok {
		result["work_history"] = member.NormalizeWorkHistory(raw)
	}
	return json.Marshal(result)
}

type registrationProfile struct {
	dbgen.Profile
	Badges           json.RawMessage `json:"badges"`
	EducationHistory json.RawMessage `json:"education_history"`
	WorkHistory      json.RawMessage `json:"work_history"`
	ExtraData        json.RawMessage `json:"extra_data"`
}
type rawProfileProjection struct {
	registrationProfile
	BirthDate *string `json:"birth_date"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}

func profileProjection(raw []byte) (map[string]json.RawMessage, error) {
	var profile *registrationProfile
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &profile); err != nil {
			return nil, err
		}
	}
	projection := rawProfileProjection{}
	if profile != nil {
		projection.registrationProfile = *profile
		projection.CreatedAt = domain.Timestamp(profile.CreatedAt, time.UTC)
		projection.UpdatedAt = domain.Timestamp(profile.UpdatedAt, time.UTC)
		if profile.BirthDate.Valid {
			date := profile.BirthDate.Time
			value := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local).UTC().Format("2006-01-02T15:04:05.000Z")
			projection.BirthDate = &value
		}
	}
	encoded, err := json.Marshal(projection)
	if err != nil {
		return nil, err
	}
	var result map[string]json.RawMessage
	if err = json.Unmarshal(encoded, &result); err != nil {
		return nil, err
	}
	if profile == nil {
		for key := range result {
			result[key] = json.RawMessage("null")
		}
	}
	return result, nil
}
func registrationSummary(row dbgen.ListRegistrationsFilteredRow, fields []string) (RegistrationSummary, error) {
	result := RegistrationSummary{ID: row.ID, UserID: row.UserID, Email: row.Email, Level: row.Level, UniversityID: row.UniversityID, ProvinceID: row.ProvinceID, IntakeYear: row.IntakeYear, Major: row.Major, Gender: row.Gender, Whatsapp: row.Whatsapp, Instagram: row.Instagram, Line: row.Line, PersonalID: row.PersonalID, EducationHistory: row.EducationHistory, GuestData: row.GuestData, Status: row.Status, CreatedAt: domain.Timestamp(row.CreatedAt, time.UTC), ProfileFields: map[string]json.RawMessage{}}
	if len(row.NameJson) > 0 {
		if err := json.Unmarshal(row.NameJson, &result.Name); err != nil {
			return result, err
		}
	}
	profile, err := profileProjection(row.Profile)
	if err != nil {
		return result, err
	}
	for _, field := range fields {
		if field == "current_education" {
			current := json.RawMessage("null")
			history := member.NormalizeEducationHistory(row.EducationHistory)
			if row.UserID == nil {
				var guest struct {
					Current json.RawMessage `json:"current_education"`
					History json.RawMessage `json:"education_history"`
				}
				_ = json.Unmarshal(row.GuestData, &guest)
				history = member.NormalizeEducationHistory(guest.History)
				if database.JSONTruthy(guest.Current) {
					current = guest.Current
				}
			}
			var entries []json.RawMessage
			if !database.JSONTruthy(current) && json.Unmarshal(history, &entries) == nil && len(entries) > 0 {
				current = entries[len(entries)-1]
			}
			result.ProfileFields[field] = current
		} else if field == "*" {
			for key, value := range profile {
				result.ProfileFields[key] = value
			}
		} else {
			result.ProfileFields[field] = profile[field]
		}
	}
	return result, nil
}
