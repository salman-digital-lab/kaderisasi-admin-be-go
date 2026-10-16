package formschema

import (
	"encoding/json"
	"strings"
)

// Match String(option.value) in the existing public controls, including numeric
// choices whose JSON representation uses a different exponent spelling.
func optionText(value any) string {
	switch value := value.(type) {
	case nil:
		return ""
	case string:
		return value
	case float64:
		if value == 0 {
			return "0"
		}
		raw, _ := json.Marshal(value)
		return string(raw)
	case bool:
		if value {
			return "true"
		}
		return "false"
	case []any:
		parts := make([]string, len(value))
		for i, item := range value {
			parts[i] = optionText(item)
		}
		return strings.Join(parts, ",")
	default:
		return "[object Object]"
	}
}

// ChoiceAnswerText shows a stored choice answer as its option labels, matching
// options by String(option.value) or by label when no value is stored. Unmatched
// values are shown as stored. It reports false for non-choice fields and answers
// that are not scalars or arrays of scalars.
func (f Field) ChoiceAnswerText(raw json.RawMessage) (string, bool) {
	switch f.Type {
	case "select", "radio", "checkbox", "multiselect":
	default:
		return "", false
	}
	if f.Options == nil {
		return "", false
	}
	labels := map[string]string{}
	for _, option := range *f.Options {
		value := option.Label
		var stored any
		if len(option.Value) > 0 && json.Unmarshal(option.Value, &stored) == nil && stored != nil && optionText(stored) != "" {
			value = optionText(stored)
		}
		if _, exists := labels[value]; !exists {
			labels[value] = option.Label
		}
	}
	var answer any
	if json.Unmarshal(raw, &answer) != nil {
		return "", false
	}
	items, isArray := answer.([]any)
	if !isArray {
		items = []any{answer}
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		switch item.(type) {
		case string, float64, bool:
		default:
			return "", false
		}
		text := optionText(item)
		if label, exists := labels[text]; exists {
			text = label
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, ", "), true
}
