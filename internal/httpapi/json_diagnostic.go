package httpapi

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"
)

// The installed Node runtime's JSON diagnostics are observable in production.
// Convert syntax categories and UTF-16 positions, without evaluating JavaScript.
func jsonDiagnostic(raw []byte, err error) string {
	syntax, ok := err.(*json.SyntaxError)
	if !ok {
		return err.Error()
	}
	text := string(raw)
	position := int(syntax.Offset) - 1
	if position < 0 {
		position = 0
	}
	if position > len(raw) {
		position = len(raw)
	}
	reason := syntax.Error()
	prefix := strings.TrimRight(text[:position], " \t\r\n")
	located := func(message string) string {
		units := utf16.Encode([]rune(text[:position]))
		line, column := 1, 1
		for _, unit := range units {
			if unit == '\n' {
				line++
				column = 1
			} else {
				column++
			}
		}
		return fmt.Sprintf("%s in JSON at position %d (line %d column %d)", message, len(units), line, column)
	}

	if strings.Contains(reason, "string escape code") {
		return located("Bad escaped character")
	}
	if strings.Contains(reason, "in \"\\u\" hexadecimal character escape") {
		return located("Bad Unicode escape")
	}
	if strings.Contains(reason, "in string literal") {
		return located("Bad control character in string literal")
	}
	if strings.Contains(reason, "numeric literal") {
		switch {
		case strings.HasSuffix(prefix, "-") && !strings.Contains(reason, "exponent"):
			return located("No number after minus sign")
		case strings.Contains(reason, "after decimal point"):
			return located("Unterminated fractional number")
		case strings.Contains(reason, "exponent"):
			return located("Exponent part is missing a number")
		}
	}
	if position > 0 && position < len(raw) && raw[position] >= '0' && raw[position] <= '9' && raw[position-1] == '0' {
		return located("Unexpected number")
	}
	if strings.Contains(reason, "after top-level value") {
		return strings.Replace(located("Unexpected non-whitespace character after JSON"), "after JSON in JSON", "after JSON", 1)
	}
	if strings.Contains(reason, "looking for beginning of object key string") {
		if strings.HasSuffix(prefix, ",") {
			return located("Expected double-quoted property name")
		}
		return located("Expected property name or '}'")
	}
	if strings.Contains(reason, "after object key") && !strings.Contains(reason, "key:value") {
		return located("Expected ':' after property name")
	}
	if strings.Contains(reason, "after object key:value pair") {
		return located("Expected ',' or '}' after property value")
	}
	if strings.Contains(reason, "after array element") {
		return located("Expected ',' or ']' after array element")
	}
	if strings.Contains(reason, "unexpected end") {
		position = len(raw)
		tail := strings.TrimRight(text, " \t\r\n")
		if strings.HasSuffix(tail, "{") {
			return located("Expected property name or '}'")
		}
		if strings.HasSuffix(tail, ",") && strings.LastIndex(tail, "{") > strings.LastIndex(tail, "[") {
			return located("Expected double-quoted property name")
		}

		inString, escape := false, false
		for _, ch := range text {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' && inString {
				escape = true
				continue
			}
			if ch == '"' {
				inString = !inString
			}
		}
		if inString {
			return located("Unterminated string")
		}
		if !strings.HasSuffix(tail, "[") && json.Valid(append(append([]byte{}, raw...), ']')) {
			return located("Expected ',' or ']' after array element")
		}
		if json.Valid(append(append([]byte{}, raw...), '}')) {
			return located("Expected ',' or '}' after property value")
		}
		return "Unexpected end of JSON input"
	}
	token := ""
	if position < len(text) {
		token = string([]rune(text[position:])[0])
	}

	snippet := "\"" + text + "\""
	units := utf16.Encode([]rune(text))
	pos := len(utf16.Encode([]rune(text[:position])))
	if len(units) > 20 {
		start, end := max(0, pos-10), min(len(units), pos+10)
		snippet = "\"" + string(utf16.Decode(units[start:end])) + "\""
		if start > 0 {
			snippet = "..." + snippet
		}
		if end < len(units) {
			snippet += "..."
		}
	}
	return fmt.Sprintf("Unexpected token '%s', %s is not valid JSON", token, snippet)
}
