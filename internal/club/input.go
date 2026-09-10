package club

import (
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/domain"
)

// Input is shared by the create and patch validators. Date fields preserve
// omission and explicit null; creation always starts closed and unpublished.
type Input struct {
	Name                *string                      `json:"name,omitempty"`
	ClubType            *string                      `json:"club_type,omitempty"`
	Description         *string                      `json:"description,omitempty"`
	ShortDescription    *string                      `json:"short_description,omitempty"`
	Media               *Media                       `json:"media,omitempty"`
	StartPeriod         domain.Optional[pgtype.Date] `json:"start_period,omitzero"`
	EndPeriod           domain.Optional[pgtype.Date] `json:"end_period,omitzero"`
	IsShow              *bool                        `json:"is_show,omitempty"`
	IsRegistrationOpen  *bool                        `json:"is_registration_open,omitempty"`
	RegistrationEndDate domain.Optional[pgtype.Date] `json:"registration_end_date,omitzero"`
}

type Created struct {
	ID                  int32       `json:"id"`
	Name                string      `json:"name"`
	ClubType            string      `json:"club_type"`
	Description         *string     `json:"description,omitempty"`
	ShortDescription    *string     `json:"short_description,omitempty"`
	Media               Media       `json:"media"`
	StartPeriod         pgtype.Date `json:"start_period"`
	EndPeriod           pgtype.Date `json:"end_period"`
	RegistrationEndDate pgtype.Date `json:"registration_end_date"`
	IsShow              bool        `json:"is_show"`
	IsRegistrationOpen  bool        `json:"is_registration_open"`
	CreatedAt           *string     `json:"created_at"`
	UpdatedAt           *string     `json:"updated_at"`
}

type RegistrationInfo struct {
	Info string `json:"registration_info"`
}
type RegistrationInfoResponse struct {
	Info RegistrationInfo `json:"registration_info"`
}

type MediaRequest struct {
	URL string `json:"media_url"`
}
type MediaResponse struct {
	Media Media `json:"media"`
}
type LogoResponse struct {
	Logo string `json:"logo"`
}

func dateValue(value domain.Optional[pgtype.Date]) pgtype.Date {
	if value.Value != nil {
		return *value.Value
	}
	return pgtype.Date{}
}
