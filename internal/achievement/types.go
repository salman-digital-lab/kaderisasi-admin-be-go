package achievement

import (
	"encoding/json"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/member"
	"time"
)

type Input struct {
	Name        *string      `json:"name"`
	Description *string      `json:"description"`
	Type        *json.Number `json:"type"`
	Score       *json.Number `json:"score"`
	Proof       *string      `json:"proof"`
	Status      *json.Number `json:"status"`
	Remark      *string      `json:"remark"`
}
type Response struct {
	dbgen.Achievement
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}
type Detail struct {
	Response
	User     *member.UserWithProfile  `json:"user"`
	Approver *auth.AdminModelResponse `json:"approver"`
}
type Filters struct {
	Status, Email, Name, Type *string
	DateOrder, Ascending      bool
	Page, Size                float64
}
type Page[T any] struct {
	Meta database.Pagination `json:"meta"`
	Data []T                 `json:"data"`
}
type LeaderboardFilters struct {
	Month, Year string
	Email, Name *string
	Page, Size  float64
}
type LeaderboardProfile struct {
	member.ProfileResponse
	University *dbgen.University `json:"university"`
}
type LeaderboardUser struct {
	member.PublicResponse
	Profile *LeaderboardProfile `json:"profile"`
}
type MonthlyResponse struct {
	dbgen.MonthlyLeaderboard
	CreatedAt *string          `json:"created_at"`
	UpdatedAt *string          `json:"updated_at"`
	User      *LeaderboardUser `json:"user"`
}
type LifetimeResponse struct {
	dbgen.LifetimeLeaderboard
	CreatedAt *string          `json:"created_at"`
	UpdatedAt *string          `json:"updated_at"`
	User      *LeaderboardUser `json:"user"`
}

func (s Service) view(row dbgen.Achievement) Response {
	return Response{Achievement: row, CreatedAt: domain.ModelTimestamp(row.CreatedAt, s.Location), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, s.Location)}
}
func (s Service) details(row dbgen.Achievement, user, profile, approver []byte, includeProfile bool) (Detail, error) {
	result := Detail{Response: s.view(row)}
	var err error
	result.User, err = member.UserFromRelation(user, profile, includeProfile)
	if err != nil {
		return result, err
	}
	result.Approver, err = auth.AdminModelFromRelation(approver, s.Location)
	return result, err
}
func leaderboardUser(userJSON, profileJSON, universityJSON []byte) (*LeaderboardUser, error) {
	user, err := member.UserFromRelation(userJSON, profileJSON, true)
	if user == nil || err != nil {
		return nil, err
	}
	result := LeaderboardUser{PublicResponse: user.PublicResponse}
	if user.Profile.Value != nil {
		result.Profile = &LeaderboardProfile{ProfileResponse: *user.Profile.Value}
		if len(universityJSON) > 0 {
			if err = json.Unmarshal(universityJSON, &result.Profile.University); err != nil {
				return nil, err
			}
		}
	}
	return &result, nil
}
func locationOrLocal(location *time.Location) *time.Location {
	if location == nil {
		return time.Local
	}
	return location
}
