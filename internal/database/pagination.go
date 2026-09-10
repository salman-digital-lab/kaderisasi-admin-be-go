package database

import (
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"
)

// PageNumber retains Lucid's metadata for fractional, zero and non-finite
// inputs. JSON.stringify serializes NaN and infinities as null.
type PageNumber float64

func (number PageNumber) MarshalJSON() ([]byte, error) {
	value := float64(number)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return []byte("null"), nil
	}
	return []byte(JSNumber(value)), nil
}

func JSNumber(value float64) string {
	if math.IsNaN(value) {
		return "NaN"
	}
	if math.IsInf(value, 1) {
		return "Infinity"
	}
	if math.IsInf(value, -1) {
		return "-Infinity"
	}
	if value == 0 {
		return "0"
	}
	if math.Abs(value) >= 1e-6 && math.Abs(value) < 1e21 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	return strings.ReplaceAll(strconv.FormatFloat(value, 'e', -1, 64), "e-0", "e-")
}

// Knex applies parseInt to the JavaScript number's decimal representation.
// In particular, a scientific-notation offset must not overflow a Go integer.
func knexInteger(value float64) *string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	text := JSNumber(value)
	if end := strings.IndexAny(text, ".e"); end >= 0 {
		text = text[:end]
	}
	return &text
}

func SQLPage(page, size float64) (*string, string, error) {
	offset := 0.0
	if page != 1 {
		offset = size * (page - 1)
	}
	start := knexInteger(offset)
	if start != nil && strings.HasPrefix(*start, "-") {
		return nil, "0", errors.New("A non-negative integer must be provided to offset.")
	}
	limit := knexInteger(size)
	if start == nil {
		return limit, "0", nil
	}
	return limit, *start, nil
}

func pageURL(page float64) string {
	return "/?page=" + url.QueryEscape(JSNumber(math.Max(page, 1)))
}

func Meta[N ~int | ~float64](total int64, page, perPage N) Pagination {
	p, size := float64(page), float64(perPage)
	last := math.Max(math.Ceil(float64(total)/size), 1)
	m := Pagination{Total: total, PerPage: PageNumber(size), CurrentPage: PageNumber(p), LastPage: PageNumber(last), FirstPage: 1, FirstPageURL: "/?page=1", LastPageURL: pageURL(last)}
	if p < last {
		next := pageURL(p + 1)
		m.NextPageURL = &next
	}
	if p > 1 {
		previous := pageURL(p - 1)
		m.PreviousPageURL = &previous
	}
	return m
}
