package member

import (
	"bytes"
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// HistoryEntries also accepts the serialized arrays left by older imports.
// Invalid elements do not prevent valid sibling entries from being displayed.
func HistoryEntries(raw json.RawMessage) []map[string]json.RawMessage {
	var serialized string
	if json.Unmarshal(raw, &serialized) == nil {
		raw = json.RawMessage(serialized)
	}
	var items []json.RawMessage
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	result := make([]map[string]json.RawMessage, 0, len(items))
	for _, item := range items {
		var entry map[string]json.RawMessage
		if json.Unmarshal(item, &entry) == nil && entry != nil {
			result = append(result, entry)
		}
	}
	return result
}

func historyString(raw json.RawMessage) string {
	var value string
	_ = json.Unmarshal(raw, &value)
	return strings.TrimSpace(value)
}

func historyYear(raw json.RawMessage) *json.Number {
	value := string(bytes.TrimSpace(raw))
	var text string
	if json.Unmarshal(raw, &text) == nil {
		value = strings.TrimSpace(text)
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
		return nil
	}
	result := json.Number(strconv.FormatFloat(number, 'f', -1, 64))
	return &result
}

func NormalizeEducationHistory(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return raw
	}
	result := []EducationEntry{}
	for _, entry := range HistoryEntries(raw) {
		value := EducationEntry{
			Institution: historyString(entry["institution"]),
			Faculty:     historyString(entry["faculty"]),
			Major:       historyString(entry["major"]),
			IntakeYear:  historyYear(entry["intake_year"]),
		}
		degree := historyString(entry["degree"])
		if degree == "bachelor" || degree == "master" || degree == "doctoral" {
			value.Degree = &degree
		}
		result = append(result, value)
	}
	encoded, _ := json.Marshal(result)
	return encoded
}

func NormalizeWorkHistory(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return raw
	}
	result := []WorkEntry{}
	for _, entry := range HistoryEntries(raw) {
		result = append(result, WorkEntry{
			JobTitle:  historyString(entry["job_title"]),
			Company:   historyString(entry["company"]),
			StartYear: historyYear(entry["start_year"]),
			EndYear:   historyYear(entry["end_year"]),
		})
	}
	encoded, _ := json.Marshal(result)
	return encoded
}
