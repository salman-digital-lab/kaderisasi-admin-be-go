package scoring

import (
	"fmt"
	"kaderisasi/admin/internal/domain"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

var identifier = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func validNumber(n float64, positive bool) bool {
	return !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 && n <= 1000000 && (!positive || n > 0) && math.Abs(n*100-math.Round(n*100)) < 0.000001
}
func ValidateDefinition(d Definition) error {
	if len(d.Groups) == 0 || len(d.Groups) > 30 || len(d.Grades) > 30 || len(d.Note) > 5000 {
		return domain.Fail(422, "INVALID_SCORING_RUBRIC")
	}
	ids := map[string]bool{}
	count := 0
	for _, g := range d.Groups {
		if !identifier.MatchString(g.ID) || ids[g.ID] || strings.TrimSpace(g.Name) == "" || len(g.Name) > 200 || len(g.Criteria) == 0 {
			return domain.Fail(422, "INVALID_SCORING_GROUP")
		}
		ids[g.ID] = true
		for _, c := range g.Criteria {
			count++
			if !identifier.MatchString(c.ID) || ids[c.ID] || strings.TrimSpace(c.Name) == "" || len(c.Name) > 200 || !validNumber(c.Maximum, true) || !validNumber(c.Weight, true) {
				return domain.Fail(422, "INVALID_SCORING_CRITERION")
			}
			ids[c.ID] = true
		}
	}
	if count > 100 {
		return domain.Fail(422, "TOO_MANY_SCORING_CRITERIA")
	}
	thresholds := map[float64]bool{}
	labels := map[string]bool{}
	for _, g := range d.Grades {
		if strings.TrimSpace(g.Label) == "" || len(g.Label) > 30 || labels[g.Label] || thresholds[g.Minimum] || !validNumber(g.Minimum, false) || g.Minimum > 100 {
			return domain.Fail(422, "INVALID_SCORING_GRADES")
		}
		thresholds[g.Minimum] = true
		labels[g.Label] = true
	}
	if len(d.Grades) > 0 && !thresholds[0] {
		return domain.Fail(422, "SCORING_GRADES_REQUIRE_ZERO")
	}
	return nil
}
func Criteria(d Definition) []Criterion {
	result := []Criterion{}
	for _, g := range d.Groups {
		result = append(result, g.Criteria...)
	}
	return result
}
func rational(n float64) *big.Rat {
	r, _ := new(big.Rat).SetString(strconv.FormatFloat(n, 'f', 2, 64))
	return r
}
func rounded(r *big.Rat) float64 { n, _ := strconv.ParseFloat(r.FloatString(2), 64); return n }
func grade(d Definition, n float64) *string {
	var result *string
	minimum := -1.0
	for _, g := range d.Grades {
		if n >= g.Minimum && g.Minimum > minimum {
			label := g.Label
			result = &label
			minimum = g.Minimum
		}
	}
	return result
}
func Calculate(d Definition, draft Draft) (Result, error) {
	result := Result{Criteria: []CriterionResult{}, Complete: true}
	if err := ValidateDefinition(d); err != nil {
		return result, err
	}
	if len(draft.Note) > 5000 {
		return result, domain.Fail(422, "SCORING_NOTE_TOO_LONG")
	}
	known := map[string]bool{}
	weighted := new(big.Rat)
	weights := new(big.Rat)
	for _, c := range Criteria(d) {
		known[c.ID] = true
		score := draft.Scores[c.ID]
		row := CriterionResult{CriterionID: c.ID, Score: score}
		if score == nil {
			result.Complete = false
		} else {
			if !validNumber(*score, false) || *score > c.Maximum {
				return result, domain.Fail(422, fmt.Sprintf("INVALID_SCORE: %s", c.ID))
			}
			normalized := new(big.Rat).Mul(new(big.Rat).Quo(rational(*score), rational(c.Maximum)), big.NewRat(100, 1))
			n := rounded(normalized)
			row.Normalized = &n
			row.Grade = grade(d, n)
			weighted.Add(weighted, new(big.Rat).Mul(normalized, rational(c.Weight)))
		}
		weights.Add(weights, rational(c.Weight))
		result.Criteria = append(result.Criteria, row)
	}
	for id := range draft.Scores {
		if !known[id] {
			return result, domain.Fail(422, "UNKNOWN_SCORING_CRITERION")
		}
	}
	if result.Complete {
		n := rounded(new(big.Rat).Quo(weighted, weights))
		result.Total = &n
		result.Grade = grade(d, n)
	}
	return result, nil
}
