package export

import (
	"encoding/json"
	"testing"
)

func TestRegistrationCurrentChoiceLabels(t *testing.T) {
	for _, custom := range []bool{true, false} {
		schema := `{"additional_questionnaire":[{"name":"choice","label":"Pilihan terbaru","options":[{"value":"Old label","label":"New label"},{"value":0,"label":"Zero"}]}]}`
		if custom {
			schema = `{"fields":[{"section_name":"Answers","fields":[{"key":"choice","label":"Pilihan terbaru","options":[{"value":"Old label","label":"New label"},{"value":0,"label":"Zero"}]}]}]}`
		}
		questions, err := RegistrationQuestions([]byte(schema), custom)
		if err != nil {
			t.Fatal(err)
		}
		for _, test := range []struct{ answer, want string }{
			{`"Old label"`, "New label"},
			{`["Old label","0","Deleted option"]`, "New label, Zero, Deleted option"},
			{`0`, "Zero"},
			{`null`, ""},
			{`"Deleted option"`, "Deleted option"},
		} {
			row, err := RegistrationRow(1, Registration{Answers: json.RawMessage(`{"choice":` + test.answer + `}`)}, questions, "")
			if err != nil {
				t.Fatal(err)
			}
			if got := row[len(RegistrationHeaders)]; got != test.want {
				t.Errorf("custom=%t answer=%s: got %v, want %s", custom, test.answer, got, test.want)
			}
		}
	}
}
