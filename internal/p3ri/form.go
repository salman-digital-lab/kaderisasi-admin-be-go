// Package p3ri reviews P3RI 1448 H committee registrations (Open Recruitment).
// Members submit them through kaderisasi-web-be-go; form_1448h.json is a
// byte-identical copy of ../kaderisasi-web-be-go/internal/p3ri/form_1448h.json
// used here for labels and the export. Change the two copies together
// (docs/P3RI.md).
package p3ri

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
)

//go:embed form_1448h.json
var formJSON []byte

// Form is the parts of the registration form the review needs. The admin
// frontend reads the raw JSON (FormJSON) for everything else.
type Form struct {
	Code            string     `json:"code"`
	Title           string     `json:"title"`
	Sections        []Section  `json:"sections"`
	ChoiceQuestions []Question `json:"choice_questions"`
	Divisions       []Division `json:"divisions"`
}

type Section struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Questions []Question `json:"questions"`
}

type Division struct {
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	Questions []Question `json:"questions"`
}

type Question struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Type  string   `json:"type"`
	Rows  []string `json:"rows,omitempty"`
}

var form = mustForm()

func mustForm() Form {
	var f Form
	if err := json.Unmarshal(formJSON, &f); err != nil {
		panic(fmt.Sprintf("p3ri: invalid form_1448h.json: %v", err))
	}
	return f
}

// Current returns the hardcoded 1448 H form.
func Current() Form { return form }

// FormJSON is the form definition as stored, for the admin frontend.
func FormJSON() json.RawMessage { return formJSON }

func (f Form) Division(code string) (Division, bool) {
	i := slices.IndexFunc(f.Divisions, func(d Division) bool { return d.Code == code })
	if i < 0 {
		return Division{}, false
	}
	return f.Divisions[i], true
}

// Statuses in pipeline order.
var Statuses = []string{"terdaftar", "diterima"}

var StatusLabels = map[string]string{
	"terdaftar": "Terdaftar",
	"diterima":  "Diterima",
}
