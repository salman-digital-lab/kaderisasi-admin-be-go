package counseling

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
	CounselorID     *json.Number `json:"counselor_id"`
	Status          *json.Number `json:"status"`
	AdditionalNotes *string      `json:"additional_notes"`
}
type Response struct {
	dbgen.RuangCurhat
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}
type Detail struct {
	Response
	PublicUser *member.UserWithProfile  `json:"publicUser"`
	AdminUser  *auth.AdminModelResponse `json:"adminUser"`
	University *dbgen.University        `json:"university"`
}
type CounselorOption struct {
	ID          int32   `json:"id"`
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name"`
}
type Filters struct {
	Status, Name, Gender, AdminName *string
	Page, Size                      float64
}
type Page struct {
	Meta database.Pagination `json:"meta"`
	Data []Detail            `json:"data"`
}

func view(row dbgen.RuangCurhat, location *time.Location) Response {
	return Response{RuangCurhat: row, CreatedAt: domain.ModelTimestamp(row.CreatedAt, location), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, location)}
}
func details(row dbgen.RuangCurhat, user, profile, admin, university []byte, location *time.Location) (Detail, error) {
	result := Detail{Response: view(row, location)}
	var err error
	result.PublicUser, err = member.UserFromRelation(user, profile, true)
	if err != nil {
		return result, err
	}
	result.AdminUser, err = auth.AdminModelFromRelation(admin, location)
	if err != nil {
		return result, err
	}
	if len(university) > 0 {
		if err = json.Unmarshal(university, &result.University); err != nil {
			return result, err
		}
	}
	return result, err
}
