package jscompat

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var decimalIdentifier = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

// Number applies JavaScript Number conversion without narrowing to a
// machine integer. Use only for actions whose source converts Number(params.id).
func Number(raw string) float64 {
	value := strings.TrimFunc(raw, Whitespace)
	if value == "" {
		return 0
	}
	if value == "Infinity" || value == "+Infinity" {
		return math.Inf(1)
	}
	if value == "-Infinity" {
		return math.Inf(-1)
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
			return f
		}
	}
	if !decimalIdentifier.MatchString(value) {
		return math.NaN()
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil && !math.IsInf(number, 0) {
		return math.NaN()
	}
	return number
}

func Whitespace(r rune) bool {
	return (r >= '\t' && r <= '\r') || r == ' ' || r == '\u00a0' || r == '\u1680' || (r >= '\u2000' && r <= '\u200a') || r == '\u2028' || r == '\u2029' || r == '\u202f' || r == '\u205f' || r == '\u3000' || r == '\ufeff'
}
