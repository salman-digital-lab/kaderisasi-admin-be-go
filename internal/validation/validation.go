package validation

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/jscompat"
	"math"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

//go:embed schemas.json
var schemasJSON []byte

type Rule struct {
	Kind  string            `json:"kind"`
	Args  []json.RawMessage `json:"args"`
	Chain []Constraint      `json:"chain"`
}
type Constraint struct {
	Method string            `json:"method"`
	Args   []json.RawMessage `json:"args"`
}
type Issue struct {
	Message string                 `json:"message"`
	Rule    string                 `json:"rule"`
	Field   string                 `json:"field"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
	Index   *int                   `json:"index,omitempty"`
}
type Object map[string]json.RawMessage

func (o Object) String(key string) string {
	var value string
	_ = json.Unmarshal(o[key], &value)
	return value
}
func (o Object) Number(key string) float64 {
	var value float64
	_ = json.Unmarshal(o[key], &value)
	return value
}
func (o Object) ID(key string) int32 {
	value := o.Number(key)
	if value < math.MinInt32 || value > math.MaxInt32 || value != math.Trunc(value) {
		return 0
	}
	return int32(value)
}
func (o Object) Bool(key string) bool {
	var value bool
	_ = json.Unmarshal(o[key], &value)
	return value
}
func (o Object) Has(key string) bool               { _, ok := o[key]; return ok }
func (o Object) Null(key string) bool              { return bytes.Equal(o[key], []byte("null")) }
func (o Object) Set(key string, value interface{}) { o[key], _ = json.Marshal(value) }

func Validate(name string, input Object) (Object, []Issue) {
	var schemas map[string]Rule
	if err := json.Unmarshal(schemasJSON, &schemas); err != nil {
		panic(err)
	}
	rule, ok := schemas[name]
	if !ok {
		panic("unknown validator: " + name)
	}
	raw, _ := json.Marshal(input)
	output, issues := validate(rule, raw, "")
	result := Object{}
	if len(output) > 0 {
		_ = json.Unmarshal(output, &result)
	}
	return result, issues
}

// ValidateField shares the ordinary Vine contract for fields accompanying a
// multipart file. File bytes are validated separately by the upload adapter.
func ValidateField(name, field string, raw json.RawMessage) (json.RawMessage, []Issue) {
	var schemas map[string]Rule
	if err := json.Unmarshal(schemasJSON, &schemas); err != nil {
		panic(err)
	}
	schema, ok := schemas[name]
	if !ok || schema.Kind != "object" {
		panic("unknown object validator: " + name)
	}
	var fields map[string]Rule
	if err := json.Unmarshal(schema.Args[0], &fields); err != nil {
		panic(err)
	}
	rule, ok := fields[field]
	if !ok {
		panic("unknown validator field: " + field)
	}
	return validate(rule, raw, field)
}
func has(rule Rule, name string) bool {
	for _, c := range rule.Chain {
		if c.Method == name {
			return true
		}
	}
	return false
}
func issue(field, rule, message string, metadata ...map[string]interface{}) []Issue {
	var meta map[string]interface{}
	if len(metadata) > 0 {
		meta = metadata[0]
	}
	name := field
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	return []Issue{{Message: strings.ReplaceAll(message, "{field}", name), Rule: rule, Field: field, Meta: meta}}
}
func marshal(value interface{}) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
func validate(rule Rule, raw json.RawMessage, field string) (json.RawMessage, []Issue) {
	if bytes.Equal(raw, []byte(`""`)) {
		raw = json.RawMessage("null")
	}
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		if has(rule, "nullable") && len(raw) > 0 {
			return json.RawMessage("null"), nil
		}
		if has(rule, "optional") {
			return nil, nil
		}
		if rule.Kind == "any" && len(raw) > 0 {
			return raw, nil
		}
		return nil, issue(field, "required", "The {field} field must be defined")
	}
	var str string
	var num float64
	var length int
	switch rule.Kind {
	case "object", "record":
		var input Object
		if err := json.Unmarshal(raw, &input); err != nil || input == nil {
			return nil, issue(field, "object", "The {field} field must be an object")
		}
		result := Object{}
		issues := []Issue{}
		if rule.Kind == "object" {
			var properties map[string]Rule
			_ = json.Unmarshal(rule.Args[0], &properties)
			keys := orderedKeys(rule.Args[0])
			for _, key := range keys {
				path := key
				if field != "" {
					path = field + "." + key
				}
				value, errors := validate(properties[key], input[key], path)
				issues = append(issues, errors...)
				if value != nil {
					result[key] = value
				}
			}
		} else {
			var element Rule
			_ = json.Unmarshal(rule.Args[0], &element)
			for _, key := range orderedKeys(raw) {
				value := input[key]
				v, errors := validate(element, value, field+"."+key)
				issues = append(issues, errors...)
				if v != nil {
					result[key] = v
				}
			}
		}
		if len(issues) > 0 {
			return nil, issues
		}
		raw = marshal(result)
		length = len(result)
	case "array":
		var input []json.RawMessage
		if err := json.Unmarshal(raw, &input); err != nil || input == nil {
			return nil, issue(field, "array", "The {field} field must be an array")
		}
		var element Rule
		_ = json.Unmarshal(rule.Args[0], &element)
		result := []json.RawMessage{}
		issues := []Issue{}
		for i, value := range input {
			path := fmt.Sprintf("%s.%d", field, i)
			v, errors := validate(element, value, path)
			for j := range errors {
				if errors[j].Field == path {
					index := i
					errors[j].Index = &index
				}
			}
			issues = append(issues, errors...)
			if v == nil {
				v = json.RawMessage("null")
			}
			result = append(result, v)
		}
		if len(issues) > 0 {
			return nil, issues
		}
		raw = marshal(result)
		length = len(result)
	case "string":
		if err := json.Unmarshal(raw, &str); err != nil {
			return nil, issue(field, "string", "The {field} field must be a string")
		}
		if has(rule, "trim") {
			str = strings.TrimFunc(str, jscompat.Whitespace)
		}
		raw = marshal(str)
		length = len(utf16.Encode([]rune(str)))
	case "number":
		num = jscompat.JSONNumber(raw)
		if math.IsNaN(num) || math.IsInf(num, 0) {
			return nil, issue(field, "number", "The {field} field must be a number")
		}
		raw = marshal(num)
	case "boolean":
		switch string(raw) {
		case "true", "1", `"true"`, `"1"`:
			raw = json.RawMessage("true")
		case "false", "0", `"false"`, `"0"`:
			raw = json.RawMessage("false")
		default:
			return nil, issue(field, "boolean", "The value must be a boolean")
		}
	case "enum":
		var allowed []json.RawMessage
		_ = json.Unmarshal(rule.Args[0], &allowed)
		found := false
		for _, v := range allowed {
			if bytes.Equal(raw, v) {
				found = true
				break
			}
		}
		if !found {
			var choices interface{}
			_ = json.Unmarshal(rule.Args[0], &choices)
			return nil, issue(field, "enum", "The selected {field} is invalid", map[string]interface{}{"choices": choices})
		}
	case "literal":
		if !bytes.Equal(raw, rule.Args[0]) {
			var expected interface{}
			_ = json.Unmarshal(rule.Args[0], &expected)
			return nil, issue(field, "literal", "The {field} field must be "+strings.Trim(string(rule.Args[0]), `"`), map[string]interface{}{"expectedValue": expected})
		}
	case "date":
		if err := json.Unmarshal(raw, &str); err != nil {
			return nil, issue(field, "date", "The {field} field must be a datetime value")
		}
		valid := false
		// All inventoried Vine date validators use the two strict default
		// formats. ISO offsets and fractional seconds require an explicit opt-in.
		for _, format := range []string{"2006-01-02", "2006-01-02 15:04:05"} {
			if parsed, err := time.ParseInLocation(format, str, time.Local); err == nil && parsed.Format(format) == str {
				raw = marshal(parsed.In(time.Local).Format("2006-01-02"))
				valid = true
				break
			}
		}
		if !valid {
			return nil, issue(field, "date", "The {field} field must be a datetime value")
		}
	case "any":
		return raw, nil
	case "file":
		return nil, issue(field, "file", "The {field} field must be a file")
	default:
		panic("unsupported validation type: " + rule.Kind)
	}
	for _, c := range rule.Chain {
		var n float64
		if len(c.Args) > 0 {
			_ = json.Unmarshal(c.Args[0], &n)
		}
		switch c.Method {
		case "optional", "nullable", "trim":
		case "transform":
			if str == "" {
				raw = json.RawMessage("null")
			}
		case "email":
			address, err := mail.ParseAddress(str)
			if err != nil || address.Address != str || !strings.Contains(str, "@") {
				return nil, issue(field, "email", "The {field} field must be a valid email address")
			}
		case "url":
			u, err := url.ParseRequestURI(str)
			if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
				return nil, issue(field, "url", "The {field} field must be a valid URL")
			}
		case "withoutDecimals":
			if num != math.Trunc(num) {
				return nil, issue(field, "withoutDecimals", "The {field} field must be an integer")
			}
		case "positive":
			if num < 0 {
				return nil, issue(field, "positive", "The {field} field must be positive")
			}
		case "minLength", "maxLength":
			if (c.Method == "minLength" && length < int(n)) || (c.Method == "maxLength" && length > int(n)) {
				unit := "characters"
				kind := c.Method
				if rule.Kind == "array" || rule.Kind == "record" {
					unit = "items"
					kind = rule.Kind + "." + kind
				}
				text := fmt.Sprintf("The {field} field must have at least %g %s", n, unit)
				if c.Method == "maxLength" {
					text = fmt.Sprintf("The {field} field must not be greater than %g %s", n, unit)
					if unit == "items" {
						text = fmt.Sprintf("The {field} field must not have more than %g items", n)
					}
				}
				bound := "min"
				if c.Method == "maxLength" {
					bound = "max"
				}
				return nil, issue(field, kind, text, map[string]interface{}{bound: n})
			}
		case "range":
			var bounds []float64
			_ = json.Unmarshal(c.Args[0], &bounds)
			if len(bounds) != 2 {
				panic("invalid range")
			}
			if num < bounds[0] || num > bounds[1] {
				return nil, issue(field, "range", fmt.Sprintf("The {field} field must be between %s and %s", strconv.FormatFloat(bounds[0], 'f', -1, 64), strconv.FormatFloat(bounds[1], 'f', -1, 64)), map[string]interface{}{"min": bounds[0], "max": bounds[1]})
			}
		default:
			panic("unsupported validation constraint: " + c.Method)
		}
	}
	return raw, nil
}
