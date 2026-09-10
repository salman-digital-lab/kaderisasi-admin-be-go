package member

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
)

type UserWithProfile struct {
	PublicResponse
	Profile domain.Optional[ProfileResponse] `json:"profile,omitzero"`
}

func ProfileFromRelation(raw []byte) (*ProfileResponse, error) {
	var row *struct {
		dbgen.Profile
		Badges           json.RawMessage `json:"badges"`
		EducationHistory json.RawMessage `json:"education_history"`
		WorkHistory      json.RawMessage `json:"work_history"`
		ExtraData        json.RawMessage `json:"extra_data"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &row); err != nil {
			return nil, err
		}
	}
	if row == nil {
		return nil, nil
	}
	row.Profile.Badges = row.Badges
	row.Profile.EducationHistory = row.EducationHistory
	row.Profile.WorkHistory = row.WorkHistory
	row.Profile.ExtraData = row.ExtraData
	result := ProfileView(row.Profile)
	return &result, nil
}
func UserFromRelation(userJSON, profileJSON []byte, withProfile bool) (*UserWithProfile, error) {
	var user *dbgen.PublicUser
	if len(userJSON) > 0 {
		if err := json.Unmarshal(userJSON, &user); err != nil {
			return nil, err
		}
	}
	if user == nil {
		return nil, nil
	}
	result := UserWithProfile{PublicResponse: PublicView(*user)}
	if withProfile {
		profile, err := ProfileFromRelation(profileJSON)
		if err != nil {
			return nil, err
		}
		result.Profile = domain.Optional[ProfileResponse]{Present: true, Value: profile}
	}
	return &result, nil
}
