package club

import (
	"kaderisasi/admin/internal/database"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var decimalIdentifier = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

// Only the legacy club update action applies Number(params.id). Read/media
// actions send the original string to PostgreSQL. Never narrow an identifier.
func updateIdentifier(raw string) string {
	value := strings.TrimFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || r == '\ufeff' })
	if value == "" {
		return "0"
	}
	if value == "Infinity" || value == "+Infinity" {
		return "Infinity"
	}
	if value == "-Infinity" {
		return value
	}
	if len(value) > 2 && value[0] == '0' && strings.ContainsRune("xXoObB", rune(value[1])) {
		base := 16
		switch value[1] {
		case 'o', 'O':
			base = 8
		case 'b', 'B':
			base = 2
		}
		if n, ok := new(big.Int).SetString(value[2:], base); ok && !strings.ContainsAny(value[2:], "_+-") {
			f, _ := new(big.Float).SetInt(n).Float64()
			return database.JSNumber(f)
		}
	}
	if !decimalIdentifier.MatchString(value) {
		return "NaN"
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil && !math.IsInf(number, 0) {
		return "NaN"
	}
	return database.JSNumber(number)
}
