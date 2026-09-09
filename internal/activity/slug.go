package activity

import (
	"golang.org/x/text/unicode/norm"
	"strings"
	"unicode"
)

func Slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(norm.NFKD.String(name))) {
		if r >= 0x300 && r <= 0x36f {
			continue
		}
		if unicode.IsSpace(r) || unicode.Is(unicode.Pd, r) || r == '-' {
			b.WriteByte('-')
		} else if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	value := b.String()
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	value = strings.Trim(value, "-")
	if value == "" {
		return "activity"
	}
	return value
}
