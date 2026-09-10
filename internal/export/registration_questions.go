package export

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Historical JSON documents bypass current form validation. Property access
// preserves the export controller's missing/null distinction and diagnostics.
func documentProperty(raw json.RawMessage, key string) (json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		kind := "null"
		if len(raw) == 0 {
			kind = "undefined"
		}
		return nil, fmt.Errorf("TypeError: Cannot read properties of %s (reading '%s')", kind, key)
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) == nil {
		return object[key], nil
	}
	return nil, nil
}

func RegistrationQuestions(raw json.RawMessage, customForm bool) ([]Question, error) {
	questions := []Question{}
	field := "additional_questionnaire"
	if customForm {
		field = "fields"
	}
	value, err := documentProperty(raw, field)
	if err != nil {
		return nil, err
	}
	var entries []json.RawMessage
	if len(value) == 0 || value[0] != '[' || json.Unmarshal(value, &entries) != nil {
		if customForm {
			return nil, fmt.Errorf("TypeError: formSchema.fields is not iterable")
		}
		if _, err := documentProperty(value, "map"); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("TypeError: questions.map is not a function")
	}
	appendQuestion := func(raw json.RawMessage, key string) error {
		label, err := documentProperty(raw, "label")
		if err != nil {
			return err
		}
		name, err := documentProperty(raw, key)
		if err != nil {
			return err
		}
		identifier := Text(name)
		if len(name) == 0 {
			identifier = "undefined"
		} else if string(name) == "null" {
			identifier = "null"
		}
		questions = append(questions, Question{Key: identifier, Label: Text(label)})
		return nil
	}
	for _, entry := range entries {
		if !customForm {
			if err = appendQuestion(entry, "name"); err != nil {
				return nil, err
			}
			continue
		}
		name, err := documentProperty(entry, "section_name")
		if err != nil {
			return nil, err
		}
		if Text(name) == "profile_data" {
			continue
		}
		fields, _ := documentProperty(entry, "fields")
		var section []json.RawMessage
		if len(fields) == 0 || fields[0] != '[' || json.Unmarshal(fields, &section) != nil {
			return nil, fmt.Errorf("TypeError: section.fields is not iterable")
		}
		for _, item := range section {
			if err = appendQuestion(item, "key"); err != nil {
				return nil, err
			}
		}
	}
	return questions, nil
}
