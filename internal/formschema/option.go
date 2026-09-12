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
