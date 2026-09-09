package httpapi

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type fieldHole struct{}

// qs defaults: dots allowed, five nested levels, 20 as the largest array index,
// 1000 parameters, combined duplicate keys and compacted sparse arrays.
func parseFields(raw string, emptyNull bool) map[string]any {
	grouped := map[string]any{}
	order := []string{}
	for index, part := range strings.SplitN(raw, "&", 1001) {
		if index >= 1000 {
			break
		}
		if part == "" {
			continue
		}
		pair := strings.SplitN(part, "=", 2)
		key, err := url.QueryUnescape(pair[0])
		if err != nil {
			key = pair[0]
		}
		value := ""
		if len(pair) > 1 {
			value, err = url.QueryUnescape(pair[1])
			if err != nil {
				value = pair[1]
			}
		}
		var item any = value
		if emptyNull && len(pair) > 1 && value == "" {
			item = nil
		}
		if prior, ok := grouped[key]; ok {
			if list, ok := prior.([]any); ok {
				grouped[key] = append(list, item)
			} else {
				grouped[key] = []any{prior, item}
			}
		} else {
			order = append(order, key)
			grouped[key] = item
		}
	}
	var result any = map[string]any{}
	for _, key := range order {
		segments := fieldPath(key)
		if len(segments) == 0 {
			continue
		}
		blocked := false
		for _, segment := range segments {
			if prototypeField(segment) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		var value any = grouped[key]
		for i := len(segments) - 1; i >= 0; i-- {
			segment := segments[i]
			if i > 0 && segment == "" {
				if _, ok := value.([]any); !ok {
					value = []any{value}
				}
			} else if index, err := strconv.Atoi(segment); i > 0 && err == nil && index >= 0 && index <= 20 && strconv.Itoa(index) == segment {
				array := make([]any, index+1)
				for n := range array {
					array[n] = fieldHole{}
				}
				array[index] = value
				value = array
			} else {
				value = map[string]any{segment: value}
			}
		}
		result = mergeFields(result, value)
	}
	compacted, _ := compactFields(result).(map[string]any)
	if compacted == nil {
		return map[string]any{}
	}
	return compacted
}
func prototypeField(key string) bool {
	switch key {
	case "__proto__", "constructor", "toString", "toLocaleString", "valueOf", "hasOwnProperty", "isPrototypeOf", "propertyIsEnumerable", "__defineGetter__", "__defineSetter__", "__lookupGetter__", "__lookupSetter__":
		return true
	}
	return false
}

var fieldDots = regexp.MustCompile(`\.([^.[\]]+)`)

func fieldPath(key string) []string {
	key = fieldDots.ReplaceAllString(key, "[$1]")
	first := strings.IndexByte(key, '[')
	if first < 0 {
		return []string{key}
	}
	path := []string{key[:first]}
	rest := key[first:]
	for depth := 0; len(rest) > 0; depth++ {
		if depth >= 5 {
			path = append(path, rest)
			break
		}
		if rest[0] != '[' {
			break
		}
		end := strings.IndexByte(rest, ']')
		if end < 0 {
			break
		}
		path = append(path, rest[1:end])
		rest = rest[end+1:]
	}
	if path[0] == "" {
		return nil
	}
	return path
}
func mergeFields(left, right any) any {
	if _, hole := left.(fieldHole); hole {
		return right
	}
	if lm, ok := left.(map[string]any); ok {
		if rm, ok := right.(map[string]any); ok {
			for key, value := range rm {
				if old, exists := lm[key]; exists {
					lm[key] = mergeFields(old, value)
				} else {
					lm[key] = value
				}
			}
			return lm
		}
		if ra, ok := right.([]any); ok {
			for i, value := range ra {
				if _, hole := value.(fieldHole); hole {
					continue
				}
				key := strconv.Itoa(i)
				if old, exists := lm[key]; exists {
					lm[key] = mergeFields(old, value)
				} else {
					lm[key] = value
				}
			}
			return lm
		}
		return []any{lm, right}
	}
	if la, ok := left.([]any); ok {
		if ra, ok := right.([]any); ok {
			for i, value := range ra {
				if _, hole := value.(fieldHole); hole {
					continue
				}
				if i >= len(la) {
					for len(la) <= i {
						la = append(la, fieldHole{})
					}
					la[i] = value
					continue
				}
				if _, hole := la[i].(fieldHole); hole {
					la[i] = value
				} else if _, object := value.(map[string]any); object {
					la[i] = mergeFields(la[i], value)
				} else {
					la = append(la, value)
				}
			}
			return la
		}
		if _, ok := right.(map[string]any); ok {
			m := map[string]any{}
			for i, value := range la {
				if _, hole := value.(fieldHole); !hole {
					m[strconv.Itoa(i)] = value
				}
			}
			return mergeFields(m, right)
		}
		return append(la, right)
	}
	if ra, ok := right.([]any); ok {
		return append([]any{left}, ra...)
	}
	return []any{left, right}
}
func compactFields(value any) any {
	switch value := value.(type) {
	case []any:
		result := []any{}
		for _, item := range value {
			if _, hole := item.(fieldHole); !hole {
				result = append(result, compactFields(item))
			}
		}
		return result
	case map[string]any:
		for key, item := range value {
			value[key] = compactFields(item)
		}
		return value
	default:
		return value
	}
}
