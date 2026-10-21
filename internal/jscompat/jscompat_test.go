package jscompat

import (
	"encoding/json"
	"math"
	"testing"
)

func TestNumberMatchesJavaScript(t *testing.T) {
	cases := map[string]float64{
		"": 0, "  ": 0, " 42 ": 42, "1e3": 1000, "-1.5": -1.5, ".5": 0.5, "5.": 5,
		"0x10": 16, "0o17": 15, "0b101": 5, "Infinity": math.Inf(1), "-Infinity": math.Inf(-1),
		"\u00A07\uFEFF": 7,
	}
	for input, want := range cases {
		if got := Number(input); got != want {
			t.Errorf("Number(%q) = %v, want %v", input, got, want)
		}
	}
	for _, input := range []string{"abc", "1_000", "0x", "0x1_0", "1e", "--1", "12px", "NaN"} {
		if got := Number(input); !math.IsNaN(got) {
			t.Errorf("Number(%q) = %v, want NaN", input, got)
		}
	}
}

func TestJSONNumberCoercion(t *testing.T) {
	cases := map[string]float64{`7`: 7, `"8"`: 8, `true`: 1, `false`: 0, `[]`: 0, `[5]`: 5, `""`: 0}
	for input, want := range cases {
		if got := JSONNumber(json.RawMessage(input)); got != want {
			t.Errorf("JSONNumber(%s) = %v, want %v", input, got, want)
		}
	}
	for _, input := range []string{``, `null`, `{}`, `[1,2]`, `"x"`} {
		if got := JSONNumber(json.RawMessage(input)); !math.IsNaN(got) {
			t.Errorf("JSONNumber(%q) = %v, want NaN", input, got)
		}
	}
}

func TestWhitespaceSet(t *testing.T) {
	for _, r := range []rune{' ', '\t', '\n', '\u00A0', '\u2003', '\u3000', '\uFEFF'} {
		if !Whitespace(r) {
			t.Errorf("%U should be whitespace", r)
		}
	}
	for _, r := range []rune{'a', '0', '\u200B', '\u0085'} {
		if Whitespace(r) {
			t.Errorf("%U should not be whitespace", r)
		}
	}
}
