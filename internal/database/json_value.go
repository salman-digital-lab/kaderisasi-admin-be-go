package database

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
)

// JSONTruthy follows the source JavaScript controller's truth test. In
// particular, "0", "false", empty arrays and empty objects are all true.
func JSONTruthy(raw json.RawMessage) bool {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) || bytes.Equal(raw, []byte("false")) {
		return false
	}
	switch raw[0] {
	case '"':
		var text string
		return json.Unmarshal(raw, &text) == nil && text != ""
	case '[', '{', 't':
		return true
	default:
		number, err := strconv.ParseFloat(string(raw), 64)
		return err == nil && number != 0
	}
}

// JSONParameter mirrors node-postgres prepareValue for unvalidated JSON values.
// Array notation matters even when PostgreSQL rejects it as an integer.
func JSONParameter(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return "NULL"
	}
	if raw[0] == '"' {
		var text string
		_ = json.Unmarshal(raw, &text)
		return text
	}
	if raw[0] == '[' {
		var values []json.RawMessage
		_ = json.Unmarshal(raw, &values)
		elements := []string{}
		for _, value := range values {
			value = bytes.TrimSpace(value)
			if bytes.Equal(value, []byte("null")) {
				elements = append(elements, "NULL")
				continue
			}
			if len(value) > 0 && value[0] == '[' {
				elements = append(elements, JSONParameter(value))
				continue
			}
			text := strings.ReplaceAll(strings.ReplaceAll(JSONParameter(value), `\`, `\\`), `"`, `\"`)
			elements = append(elements, `"`+text+`"`)
		}
		return "{" + strings.Join(elements, ",") + "}"
	}
	if raw[0] != '{' && raw[0] != 't' && raw[0] != 'f' {
		if number, err := strconv.ParseFloat(string(raw), 64); err == nil {
			return JSNumber(number)
		}
	}
	var compact bytes.Buffer
	if json.Compact(&compact, raw) == nil {
		return compact.String()
	}
	return string(raw)
}
