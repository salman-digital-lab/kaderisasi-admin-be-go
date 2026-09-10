package member

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"time"
)

// JSON columns retain legacy strings and arbitrary form data, while the row,
// dates and relations have explicit types. Shadow generated []byte/time fields
// so encoding/json emits the existing API representation.
type ProfileResponse struct {
	dbgen.Profile
	BirthDate        *string         `json:"birth_date"`
	Badges           []string        `json:"badges"`
	EducationHistory json.RawMessage `json:"education_history"`
	WorkHistory      json.RawMessage `json:"work_history"`
	ExtraData        json.RawMessage `json:"extra_data"`
	CreatedAt        *string         `json:"created_at"`
	UpdatedAt        *string         `json:"updated_at"`
}

type ProfileSummary struct {
	ProfileResponse
	PublicUser *PublicResponse `json:"publicUser"`
}

type ProfileDetail struct {
	ProfileSummary
	Province   *dbgen.Province   `json:"province"`
	City       *dbgen.City       `json:"city"`
	University *dbgen.University `json:"university"`
}

func ProfileView(row dbgen.Profile) ProfileResponse {
	var birthDate *string
	if row.BirthDate.Valid {
		date := row.BirthDate.Time
		text := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local).UTC().Format("2006-01-02T15:04:05.000Z")
		birthDate = &text
	}
	return ProfileResponse{Profile: row, BirthDate: birthDate, Badges: normalizeBadges(row.Badges), EducationHistory: row.EducationHistory, WorkHistory: row.WorkHistory, ExtraData: row.ExtraData, CreatedAt: memberTime(row.CreatedAt), UpdatedAt: memberTime(row.UpdatedAt)}
}

func ProfileWithUser(row dbgen.Profile, rawUser []byte) (ProfileSummary, error) {
	result := ProfileSummary{ProfileResponse: ProfileView(row)}
	var user *dbgen.PublicUser
	if len(rawUser) > 0 {
		if err := json.Unmarshal(rawUser, &user); err != nil {
			return result, err
		}
	}
	if user != nil {
		view := PublicView(*user)
		result.PublicUser = &view
	}
	return result, nil
}

func ProfileWithRelations(row dbgen.ProfileDetailsByIdentifierRow) (ProfileDetail, error) {
	summary, err := ProfileWithUser(row.Profile, row.PublicUser)
	result := ProfileDetail{ProfileSummary: summary}
	if err != nil {
		return result, err
	}
	for _, relation := range []struct {
		raw    []byte
		target interface{}
	}{
		{row.Province, &result.Province}, {row.City, &result.City}, {row.University, &result.University},
	} {
		if len(relation.raw) > 0 {
			if err = json.Unmarshal(relation.raw, relation.target); err != nil {
				return result, err
			}
		}
	}
	return result, nil
}
