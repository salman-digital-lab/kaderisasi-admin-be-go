package formschema

import "encoding/json"

type Schema struct {
	Version *int      `json:"version,omitempty"`
	Fields  []Section `json:"fields"`
}
type Section struct {
	ID          *string            `json:"id,omitempty"`
	Description *string            `json:"description,omitempty"`
	Navigation  *SectionNavigation `json:"navigation,omitempty"`
	Name        string             `json:"section_name"`
	Fields      []Field            `json:"fields"`
}
type Destination struct {
	Type      string `json:"type"`
	SectionID string `json:"sectionId,omitempty"`
}
type SectionNavigation struct {
	DefaultTarget Destination   `json:"defaultTarget"`
	QuestionKey   *string       `json:"questionKey,omitempty"`
	Routes        []AnswerRoute `json:"routes,omitempty"`
}
type AnswerRoute struct {
	OptionValue string      `json:"optionValue"`
	Target      Destination `json:"target"`
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
