package httpapi

import (
	"encoding/json"
	"errors"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/validation"
	"math"
	"net/http"
	"strconv"
)

func (s *Server) queries() database.JSONQueries { return database.JSONQueries{DB: s.Pool} }
func input(w http.ResponseWriter, r *http.Request, schema string) (validation.Object, bool) {
	return validatedInput(w, r, schema, false)
}
func validatedInput(w http.ResponseWriter, r *http.Request, schema string, legacyErrors bool) (validation.Object, bool) {
	style := "unhandled"
	if legacyErrors {
		style = "detailed"
	}
	return inputWithErrors(w, r, schema, style)
}

// These controllers catch Vine's exception as an ordinary Error. Its message,
// status, and envelope are part of their existing API contract.
func caughtValidationInput(w http.ResponseWriter, r *http.Request, schema string) (validation.Object, bool) {
	return inputWithErrors(w, r, schema, "caught")
}

func inputWithErrors(w http.ResponseWriter, r *http.Request, schema, style string) (validation.Object, bool) {
	body := requestData(r)
	validated, issues := validation.Validate(schema, body)
	if len(issues) > 0 {
		if style == "achievement" {
			achievementMissing(w, errors.New("Validation failure"))
			return nil, false
		}
		if style == "caught" {
			legacyFailure(w, errors.New("Validation failure"))
			return nil, false
		}
		if style == "detailed" {
			write(w, 500, struct {
				Message string             `json:"message"`
				Error   []validation.Issue `json:"error"`
			}{issues[0].Message, issues})
			return nil, false
		}
		write(w, 422, struct {
			Errors []validation.Issue `json:"errors"`
		}{issues})
		return nil, false
	}
	return validated, true
}
func pathID(r *http.Request, key string) int32 {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 32)
	if err != nil {
		return 0
	}
	return int32(id)
}
func queryNumber(r *http.Request, key string, fallback float64) float64 {
	if !r.URL.Query().Has(key) {
		return fallback
	}
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return math.NaN()
	}
	return number
}

func pageParams(r *http.Request, defaultPerPage, maxPerPage int) (float64, float64) {
	page := queryNumber(r, "page", 1)
	perPage := queryNumber(r, "per_page", float64(defaultPerPage))
	if maxPerPage > 0 {
		perPage = math.Min(perPage, float64(maxPerPage))
	}
	return page, perPage
}

func boundedPageParams(r *http.Request, defaultPerPage int) (float64, float64) {
	page, size := pageParams(r, defaultPerPage, 0)
	if page == 0 || math.IsNaN(page) {
		page = 1
	}
	if size == 0 || math.IsNaN(size) {
		size = float64(defaultPerPage)
	}
	return math.Max(page, 1), math.Min(math.Max(size, 1), 100)
}

// Validation retains Vine coercion and issue ordering. The handler receives an
// explicit DTO only after those rules succeed; typed queries own DB conversion.
func inputAs[T any](w http.ResponseWriter, r *http.Request, schema string) (T, bool) {
	return validatedInputAs[T](w, r, schema, false)
}

func validatedInputAs[T any](w http.ResponseWriter, r *http.Request, schema string, legacyErrors bool) (T, bool) {
	var result T
	data, ok := validatedInput(w, r, schema, legacyErrors)
	if !ok {
		return result, false
	}
	return decodeInputAs[T](w, data)
}

func caughtInputAs[T any](w http.ResponseWriter, r *http.Request, schema string) (T, bool) {
	data, ok := caughtValidationInput(w, r, schema)
	if !ok {
		var result T
		return result, false
	}
	return decodeInputAs[T](w, data)
}

func decodeInputAs[T any](w http.ResponseWriter, data validation.Object) (T, bool) {
	var result T
	raw, err := json.Marshal(data)
	if err == nil {
		err = json.Unmarshal(raw, &result)
	}
	if err != nil {
		message(w, 500, "GENERAL_ERROR")
		return result, false
	}
	return result, true
}
