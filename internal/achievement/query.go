package achievement

import (
	"errors"
	"fmt"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var integerPrefix = regexp.MustCompile(`^[+-]?(?:0[xX][0-9a-fA-F]+|[0-9]+)`)

func parseInteger(raw string) float64 {
	text := integerPrefix.FindString(strings.TrimSpace(raw))
	if text == "" {
		return math.NaN()
	}
	sign := 1.0
	if text[0] == '-' {
		sign = -1
		text = text[1:]
	} else if text[0] == '+' {
		text = text[1:]
	}
	value, _ := strconv.ParseFloat(database.NumberIdentifier(text), 64)
	return sign * value
}
func sqlDate(year, month, day int) *string {
	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if math.Abs(float64(date.UnixMilli())) > 8640000000000000 {
		return nil
	}
	yearText := fmt.Sprintf("%04d", year)
	if year < 0 {
		yearText = fmt.Sprintf("-%06d", -year)
	} else if year > 9999 {
		yearText = fmt.Sprintf("+%06d", year)
	}
	text := fmt.Sprintf("%s-%02d-%02d", yearText, month, day)
	return &text
}
func monthFilters(filters LeaderboardFilters) (dbgen.CountMonthlyLeaderboardParams, error) {
	result := dbgen.CountMonthlyLeaderboardParams{Email: filters.Email, Name: filters.Name}
	if filters.Year == "" {
		return result, nil
	}
	year := parseInteger(filters.Year)
	if math.IsNaN(year) || math.IsInf(year, 0) {
		return result, errors.New("Invalid unit value " + database.JSNumber(year))
	}
	if filters.Month != "" {
		result.FilterMonth = true
		month := parseInteger(filters.Month)
		if math.IsNaN(month) || math.IsInf(month, 0) {
			return result, errors.New("Invalid unit value " + database.JSNumber(month))
		}
		if month >= 1 && month <= 12 && year >= -271821 && year <= 275760 {
			result.Month = sqlDate(int(year), int(month), 1)
		}
	} else {
		result.FilterYear = true
		if year >= -271821 && year <= 275760 {
			result.StartDate = sqlDate(int(year), 1, 1)
			result.EndDate = sqlDate(int(year), 12, 31)
		}
	}
	return result, nil
}
func queryStatement(table string, conditions []string, n int, count bool, page float64, sort, direction string) string {
	projection := "*"
	if count {
		projection = `count(*) as "total"`
	}
	statement := `select ` + projection + ` from "` + table + `"`
	if len(conditions) > 0 {
		statement += " where " + strings.Join(conditions, " and ")
	}
	if !count {
		n++
		statement += fmt.Sprintf(` order by "%s" %s limit $%d`, sort, direction, n)
		if page != 1 {
			n++
			statement += fmt.Sprintf(" offset $%d", n)
		}
	}
	return statement
}
func achievementQuery(filters Filters, count bool) string {
	conditions := []string{}
	n := 0
	arg := func() string { n++; return "$" + strconv.Itoa(n) }
	if filters.Status != nil {
		conditions = append(conditions, `"status" = `+arg())
	}
	if filters.Email != nil {
		conditions = append(conditions, `exists (select * from "public_users" where ("email" = `+arg()+`) and ("public_users"."id" = "achievements"."user_id"))`)
	}
	if filters.Name != nil {
		conditions = append(conditions, `exists (select * from "public_users" where (exists (select * from "profiles" where ("name" ilike `+arg()+`) and ("public_users"."id" = "profiles"."user_id"))) and ("public_users"."id" = "achievements"."user_id"))`)
	}
	if filters.Type != nil {
		conditions = append(conditions, `"type" = `+arg())
	}
	sort, direction := "created_at", "desc"
	if filters.DateOrder {
		sort = "achievement_date"
	}
	if filters.Ascending {
		direction = "asc"
	}
	return queryStatement("achievements", conditions, n, count, filters.Page, sort, direction)
}
func leaderboardQuery(filters LeaderboardFilters, month *dbgen.CountMonthlyLeaderboardParams, count bool) string {
	table := "lifetime_leaderboards"
	conditions := []string{}
	n := 0
	arg := func() string { n++; return "$" + strconv.Itoa(n) }
	if month != nil {
		table = "monthly_leaderboards"
		if month.FilterMonth {
			if month.Month == nil {
				conditions = append(conditions, `"month" is null`)
			} else {
				conditions = append(conditions, `"month" = `+arg())
			}
		} else if month.FilterYear {
			conditions = append(conditions, `"month" between `+arg()+` and `+arg())
		}
	}
	if filters.Email != nil {
		conditions = append(conditions, `exists (select * from "public_users" where ("email" ilike `+arg()+`) and ("public_users"."id" = "`+table+`"."user_id"))`)
	}
	if filters.Name != nil {
		conditions = append(conditions, `exists (select * from "public_users" where (exists (select * from "profiles" where ("name" ilike `+arg()+`) and ("public_users"."id" = "profiles"."user_id"))) and ("public_users"."id" = "`+table+`"."user_id"))`)
	}
	return queryStatement(table, conditions, n, count, filters.Page, "score", "desc")
}
