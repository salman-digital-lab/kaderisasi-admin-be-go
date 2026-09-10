package export

import (
	"encoding/json"
	"kaderisasi/admin/internal/member"
)

type Document struct {
	Filename string
	Body     []byte
}
type EducationEntry struct {
	Degree      json.RawMessage `json:"degree"`
	Institution json.RawMessage `json:"institution"`
	Faculty     json.RawMessage `json:"faculty"`
	Major       json.RawMessage `json:"major"`
	IntakeYear  json.RawMessage `json:"intake_year"`
}
type WorkEntry struct {
	JobTitle  json.RawMessage `json:"job_title"`
	Company   json.RawMessage `json:"company"`
	StartYear json.RawMessage `json:"start_year"`
	EndYear   json.RawMessage `json:"end_year"`
}

// Guest values originate in historical public registration forms, which allow
// strings and JSON numbers for identity and location fields.
type RegistrationGuest struct {
	Name             json.RawMessage `json:"name"`
	Email            json.RawMessage `json:"email"`
	Whatsapp         json.RawMessage `json:"whatsapp"`
	BirthDate        json.RawMessage `json:"birth_date"`
	Country          json.RawMessage `json:"country"`
	Major            json.RawMessage `json:"major"`
	IntakeYear       json.RawMessage `json:"intake_year"`
	ProvinceID       json.RawMessage `json:"province_id"`
	CityID           json.RawMessage `json:"city_id"`
	OriginProvinceID json.RawMessage `json:"origin_province_id"`
	OriginCityID     json.RawMessage `json:"origin_city_id"`
	UniversityID     json.RawMessage `json:"university_id"`
	EducationHistory json.RawMessage `json:"education_history"`
	WorkHistory      json.RawMessage `json:"work_history"`
	CurrentEducation json.RawMessage `json:"current_education"`
}
type RegistrationLocations struct {
	Province       *string `json:"province"`
	City           *string `json:"city"`
	OriginProvince *string `json:"origin_province"`
	OriginCity     *string `json:"origin_city"`
	University     *string `json:"university"`
}
type Registration struct {
	ID        int32
	UserID    *int32
	Email     *string
	Guest     RegistrationGuest
	Profile   *member.ProfileResponse
	Locations RegistrationLocations
	Answers   json.RawMessage
}
