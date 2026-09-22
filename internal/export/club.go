package export

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var separators = regexp.MustCompile(`[._-]+`)

func humanize(key string) string {
	value := strings.Join(strings.Fields(separators.ReplaceAllString(key, " ")), " ")
	if value == "" {
		return "Pertanyaan"
	}
	r, size := utf8.DecodeRuneInString(value)
	return string(unicode.ToUpper(r)) + value[size:]
}

// ObjectKeys keeps PostgreSQL's serialized field order while matching JavaScript
// integer-index ordering. Sorted Go map iteration would change export columns.
func ObjectKeys(raw []byte) []string {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil
	}
	keys := []string{}
	numeric := []uint64{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil
		}
		key, ok := token.(string)
		if !ok {
			return nil
		}
		var value json.RawMessage
		if decoder.Decode(&value) != nil {
			return nil
		}
		if n, err := strconv.ParseUint(key, 10, 32); err == nil && n < 4294967295 && strconv.FormatUint(n, 10) == key {
			numeric = append(numeric, n)
		} else {
			keys = append(keys, key)
		}
	}
	slices.Sort(numeric)
	out := []string{}
	for _, n := range numeric {
		out = append(out, strconv.FormatUint(n, 10))
	}
	return append(out, keys...)
}

type ClubField struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Options []struct {
		Value json.RawMessage `json:"value"`
		Label string          `json:"label"`
	} `json:"options,omitempty"`
}

type clubFormSchema struct {
	Fields []struct {
		SectionName string      `json:"section_name"`
		Fields      []ClubField `json:"fields"`
	} `json:"fields"`
}

func ClubQuestions(schema []byte, answerSets [][]byte) []ClubField {
	var form clubFormSchema
	_ = json.Unmarshal(schema, &form)
	result := []ClubField{}
	seen := map[string]bool{}
	for _, section := range form.Fields {
		if section.SectionName == "profile_data" {
			for _, field := range section.Fields {
				seen[field.Key] = true
			}
		}
	}
	for _, section := range form.Fields {
		if section.SectionName == "profile_data" {
			continue
		}
		for _, field := range section.Fields {
			if !seen[field.Key] {
				seen[field.Key] = true
				result = append(result, field)
			}
		}
	}
	for _, answers := range answerSets {
		for _, key := range ObjectKeys(answers) {
			if !seen[key] {
				seen[key] = true
				result = append(result, ClubField{Key: key, Label: humanize(key)})
			}
		}
	}
	return result
}

func (field ClubField) Answer(raw json.RawMessage) interface{} {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ""
	}
	var array []json.RawMessage
	if json.Unmarshal(raw, &array) == nil {
		parts := make([]string, len(array))
		for i, value := range array {
			parts[i] = fmt.Sprint(field.Answer(value))
		}
		return strings.Join(parts, ", ")
	}
	for _, option := range field.Options {
		if Text(raw) == Text(option.Value) {
			return option.Label
		}
	}
	return ClubAnswer(raw)
}
func ClubAnswer(raw json.RawMessage) interface{} {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	switch string(raw) {
	case "true":
		return "Ya"
	case "false":
		return "Tidak"
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text
	}
	var number float64
	if json.Unmarshal(raw, &number) == nil {
		return number
	}
	var array []json.RawMessage
	if json.Unmarshal(raw, &array) == nil {
		parts := make([]string, len(array))
		for i, value := range array {
			parts[i] = fmt.Sprint(ClubAnswer(value))
		}
		return strings.Join(parts, ", ")
	}
	return Text(raw)
}
