package form

import (
	"encoding/json"
	"kaderisasi/admin/internal/domain"
)

type Input struct {
	FormName           *string                      `json:"formName,omitempty"`
	FormDescription    domain.Optional[string]      `json:"formDescription,omitzero"`
	PostSubmissionInfo domain.Optional[string]      `json:"postSubmissionInfo,omitzero"`
	FeatureType        *string                      `json:"featureType,omitempty"`
	FeatureID          domain.Optional[json.Number] `json:"featureId,omitzero"`
	FormSchema         *Schema                      `json:"formSchema,omitempty"`
	IsActive           *bool                        `json:"isActive,omitempty"`
}

type Schema struct {
	Fields []Section `json:"fields"`
}
type Section struct {
	Name   string  `json:"section_name"`
	Fields []Field `json:"fields"`
}
type Field struct {
	Key          string           `json:"key"`
	Label        string           `json:"label"`
	Required     bool             `json:"required"`
	Type         string           `json:"type"`
	Placeholder  *string          `json:"placeholder,omitempty"`
	HelpText     *string          `json:"helpText,omitempty"`
	Description  *string          `json:"description,omitempty"`
	Options      *[]Option        `json:"options,omitempty"`
	Validation   *FieldValidation `json:"validation,omitempty"`
	DefaultValue json.RawMessage  `json:"defaultValue,omitempty"`
	Hidden       *bool            `json:"hidden,omitempty"`
	Disabled     *bool            `json:"disabled,omitempty"`
}
type Option struct {
	Label    string          `json:"label"`
	Value    json.RawMessage `json:"value,omitempty"`
	Disabled *bool           `json:"disabled,omitempty"`
}
type FieldValidation struct {
	Min           *json.Number `json:"min,omitempty"`
	Max           *json.Number `json:"max,omitempty"`
	MinLength     *json.Number `json:"minLength,omitempty"`
	MaxLength     *json.Number `json:"maxLength,omitempty"`
	Pattern       *string      `json:"pattern,omitempty"`
	CustomMessage *string      `json:"customMessage,omitempty"`
}
type ClubAttachment struct {
	ClubID json.Number `json:"clubId"`
}

// The activity attachment action deliberately has no validator in Adonis.
// Preserve the submitted JSON type in its immediate response, even when the
// database stores a numeric string as an integer.
type ActivityAttachment struct {
	ActivityID json.RawMessage `json:"activityId"`
}
type ActivityAttached struct {
	Response
	FeatureID json.RawMessage `json:"feature_id"`
}

// Creation returns only attributes assigned on the new Lucid model; defaults
// become visible on subsequent reads. Optional also retains explicit nulls.
type Created struct {
	ID                 int32                        `json:"id"`
	FormName           string                       `json:"form_name"`
	FormDescription    domain.Optional[string]      `json:"form_description,omitzero"`
	PostSubmissionInfo domain.Optional[string]      `json:"post_submission_info,omitzero"`
	FeatureType        *string                      `json:"feature_type,omitempty"`
	FeatureID          domain.Optional[json.Number] `json:"feature_id,omitzero"`
	FormSchema         *Schema                      `json:"form_schema,omitempty"`
	IsActive           *bool                        `json:"is_active,omitempty"`
	CreatedAt          *string                      `json:"created_at"`
	UpdatedAt          *string                      `json:"updated_at"`
}
