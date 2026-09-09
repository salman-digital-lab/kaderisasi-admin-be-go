package activity

import (
	"encoding/json"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/domain"
)

type ProfileRequirement struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
}
type StatusVisibility struct {
	IsVisible bool    `json:"is_visible"`
	VisibleAt *string `json:"visible_at,omitempty"`
}
type AdditionalConfig struct {
	CustomSelectionStatus   []string                     `json:"custom_selection_status"`
	MandatoryProfileData    []ProfileRequirement         `json:"mandatory_profile_data"`
	AdditionalQuestionnaire []json.RawMessage            `json:"additional_questionnaire"`
	StatusVisibility        *StatusVisibility            `json:"status_visibility,omitempty"`
	CertificateTemplateID   domain.Optional[json.Number] `json:"certificate_template_id,omitzero"`
	AllowGuestRegistration  *bool                        `json:"allow_guest_registration,omitempty"`
}
type Input struct {
	Name                  *string                      `json:"name,omitempty"`
	Description           *string                      `json:"description,omitempty"`
	Badge                 *string                      `json:"badge,omitempty"`
	ActivityStart         *string                      `json:"activity_start,omitempty"`
	ActivityEnd           *string                      `json:"activity_end,omitempty"`
	RegistrationStart     *string                      `json:"registration_start,omitempty"`
	RegistrationEnd       *string                      `json:"registration_end,omitempty"`
	SelectionStart        *string                      `json:"selection_start,omitempty"`
	SelectionEnd          *string                      `json:"selection_end,omitempty"`
	MinimumLevel          *json.Number                 `json:"minimum_level,omitempty"`
	ActivityType          *json.Number                 `json:"activity_type,omitempty"`
	ActivityCategory      *json.Number                 `json:"activity_category,omitempty"`
	IsPublished           *json.Number                 `json:"is_published,omitempty"`
	IsRegistrationOpen    *bool                        `json:"is_registration_open,omitempty"`
	ClubID                domain.Optional[json.Number] `json:"club_id,omitzero"`
	CertificateTemplateID domain.Optional[json.Number] `json:"certificate_template_id,omitzero"`
	AdditionalConfig      *AdditionalConfig            `json:"additional_config,omitempty"`
}
type Created struct {
	Input
	ID        int32   `json:"id"`
	Slug      string  `json:"slug"`
	CreatedAt *string `json:"created_at"`
	UpdatedAt *string `json:"updated_at"`
}
type Updated struct {
	Response
	IsPublished       json.RawMessage `json:"is_published"`
	ActivityStart     *pgtype.Date    `json:"activity_start,omitempty"`
	ActivityEnd       *pgtype.Date    `json:"activity_end,omitempty"`
	RegistrationStart *pgtype.Date    `json:"registration_start,omitempty"`
	RegistrationEnd   *pgtype.Date    `json:"registration_end,omitempty"`
	SelectionStart    *pgtype.Date    `json:"selection_start,omitempty"`
	SelectionEnd      *pgtype.Date    `json:"selection_end,omitempty"`
}
