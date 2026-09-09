package member

import "encoding/json"

type CredentialRequest struct {
	Email    *string `json:"email,omitempty"`
	Password *string `json:"password,omitempty"`
}

type EducationEntry struct {
	Degree      string      `json:"degree"`
	Institution string      `json:"institution"`
	Faculty     string      `json:"faculty"`
	Major       string      `json:"major"`
	IntakeYear  json.Number `json:"intake_year"`
}
type WorkEntry struct {
	JobTitle  string       `json:"job_title"`
	Company   string       `json:"company"`
	StartYear *json.Number `json:"start_year,omitempty"`
	EndYear   *json.Number `json:"end_year,omitempty"`
}
type ProfileExtra struct {
	PreferredName            *string   `json:"preferred_name,omitempty"`
	SalmanActivityHistory    *[]string `json:"salman_activity_history,omitempty"`
	CurrentActivityFocus     *[]string `json:"current_activity_focus,omitempty"`
	AlumniRegionalAssignment *[]string `json:"alumni_regional_assignment,omitempty"`
}
type RegionalRequest struct {
	AlumniRegionalAssignment []string `json:"alumni_regional_assignment"`
}

// Vine's optional (non-nullable) fields omit both missing and explicit null.
// Pointers retain every validated value, including empty arrays and zero.
type ProfileUpdate struct {
	Name             *string           `json:"name,omitempty"`
	Gender           *string           `json:"gender,omitempty"`
	PersonalID       *string           `json:"personal_id,omitempty"`
	Whatsapp         *string           `json:"whatsapp,omitempty"`
	Line             *string           `json:"line,omitempty"`
	Instagram        *string           `json:"instagram,omitempty"`
	Tiktok           *string           `json:"tiktok,omitempty"`
	Linkedin         *string           `json:"linkedin,omitempty"`
	ProvinceID       *json.Number      `json:"province_id,omitempty"`
	CityID           *json.Number      `json:"city_id,omitempty"`
	Level            *json.Number      `json:"level,omitempty"`
	BirthDate        *string           `json:"birth_date,omitempty"`
	OriginProvinceID *json.Number      `json:"origin_province_id,omitempty"`
	OriginCityID     *json.Number      `json:"origin_city_id,omitempty"`
	Country          *string           `json:"country,omitempty"`
	Badges           *[]string         `json:"badges,omitempty"`
	EducationHistory *[]EducationEntry `json:"education_history,omitempty"`
	WorkHistory      *[]WorkEntry      `json:"work_history,omitempty"`
	ExtraData        *ProfileExtra     `json:"extra_data,omitempty"`
	Password         *string           `json:"password,omitempty"`
}
