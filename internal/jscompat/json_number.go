package jscompat

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
)

// JSONNumber implements Vine's Number coercion; Vine treats null as NaN.
func JSONNumber(raw json.RawMessage) float64 {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return math.NaN()
	}
	switch string(raw) {
	case "true":
		return 1
	case "false":
		return 0
	}
	return Number(jsonPrimitiveText(raw))
}

func jsonPrimitiveText(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	if raw[0] == '"' {
		var value string
		_ = json.Unmarshal(raw, &value)
		return value
	}
	if raw[0] == '{' {
		return "[object Object]"
	}
	if raw[0] == '[' {
		var values []json.RawMessage
		_ = json.Unmarshal(raw, &values)
		parts := make([]string, len(values))
		for i, value := range values {
			parts[i] = jsonPrimitiveText(value)
		}
		return strings.Join(parts, ",")
	}
	return string(raw)
}
