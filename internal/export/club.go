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
func ClubQuestions(schema []byte, answerSets [][]byte) []Question {
	result := []Question{}
	seen := map[string]bool{}
	for _, q := range FormQuestions(schema) {
		if !seen[q.Key] {
			seen[q.Key] = true
			result = append(result, q)
		}
	}
	for _, answers := range answerSets {
		for _, key := range ObjectKeys(answers) {
			if !seen[key] {
				seen[key] = true
				result = append(result, Question{Key: key, Label: humanize(key)})
			}
		}
	}
	return result
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
