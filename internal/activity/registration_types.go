package activity

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"time"
)

type RegistrationInput struct {
	ProfileID           json.Number                `json:"user_id"`
	QuestionnaireAnswer map[string]json.RawMessage `json:"questionnaire_answer"`
}
type RegistrationStatusInput struct {
	Status string        `json:"status"`
	IDs    []json.Number `json:"registrations_id"`
}
type RegistrationEmailStatusInput struct {
	Status string   `json:"status"`
	Emails []string `json:"emails"`
}
type RegistrationBulkStatusInput struct {
	Name          *string `json:"name"`
	CurrentStatus *string `json:"current_status"`
	NewStatus     string  `json:"new_status"`
}
type RegistrationResponse struct {
	dbgen.ActivityRegistration
	QuestionnaireAnswer json.RawMessage `json:"questionnaire_answer"`
	GuestData           json.RawMessage `json:"guest_data"`
	CreatedAt           *string         `json:"created_at"`
	UpdatedAt           *string         `json:"updated_at"`
}
type CreatedRegistration struct {
	ID                  int32           `json:"id"`
	UserID              *int32          `json:"user_id"`
	ActivityID          *int32          `json:"activity_id"`
	Status              *string         `json:"status"`
	QuestionnaireAnswer json.RawMessage `json:"questionnaire_answer"`
	CreatedAt           *string         `json:"created_at"`
	UpdatedAt           *string         `json:"updated_at"`
}
type UserRegistration struct {
	RegistrationResponse
	Activity *Response `json:"activity"`
}
type RegistrationStatistics struct {
	Total    int64            `json:"total"`
	ByStatus map[string]int64 `json:"by_status"`
}

func RegistrationView(row dbgen.ActivityRegistration) RegistrationResponse {
	return RegistrationResponse{ActivityRegistration: row, QuestionnaireAnswer: row.QuestionnaireAnswer, GuestData: row.GuestData, CreatedAt: domain.ModelTimestamp(row.CreatedAt, time.Local), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, time.Local)}
}
func CreatedRegistrationView(row dbgen.ActivityRegistration) CreatedRegistration {
	return CreatedRegistration{ID: row.ID, UserID: row.UserID, ActivityID: row.ActivityID, Status: row.Status, QuestionnaireAnswer: row.QuestionnaireAnswer, CreatedAt: domain.ModelTimestamp(row.CreatedAt, time.Local), UpdatedAt: domain.ModelTimestamp(row.UpdatedAt, time.Local)}
}
