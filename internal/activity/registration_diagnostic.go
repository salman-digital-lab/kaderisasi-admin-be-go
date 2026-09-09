package activity

import (
	"fmt"
	"strings"
)

// Lucid exposes this query string in controller errors. It is a diagnostic
// projection of the source statement, never an executable query or raw input.
func registrationListStatement(filters RegistrationFilters, fields []string, count bool) string {
	quote := func(name string) string {
		parts := strings.Split(name, ".")
		for i, part := range parts {
			if part != "*" {
				parts[i] = `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
			}
		}
		return strings.Join(parts, ".")
	}
	columns := []string{`"activity_registrations"."id"`, `"public_users"."id" as "user_id"`, "COALESCE(public_users.email, activity_registrations.guest_data->>'email') as email", "COALESCE(profiles.name, activity_registrations.guest_data->>'name') as name", `"profiles"."level"`, `"profiles"."university_id"`, `"profiles"."province_id"`, `"profiles"."intake_year"`, `"profiles"."major"`, "COALESCE(profiles.gender, activity_registrations.guest_data->>'gender') as gender", "COALESCE(profiles.whatsapp, activity_registrations.guest_data->>'whatsapp') as whatsapp", `"profiles"."instagram"`, `"profiles"."line"`, `"profiles"."personal_id"`, `"profiles"."education_history"`, `"activity_registrations"."guest_data"`}
	for _, field := range fields {
		columns = append(columns, quote("profiles."+field))
	}
	columns = append(columns, `"activity_registrations"."status"`, `"activity_registrations"."created_at"`)
	statement := "select " + strings.Join(columns, ", ") + ` from "activity_registrations" left join "public_users" on "activity_registrations"."user_id" = "public_users"."id" left join "profiles" on "activity_registrations"."user_id" = "profiles"."user_id" where "activity_registrations"."activity_id" = $1`
	index := 1
	arg := func() string { index++; return fmt.Sprintf("$%d", index) }
	if filters.Search != nil {
		statement += ` and ("profiles"."name" ilike ` + arg() + ` or "public_users"."email" ilike ` + arg() + " or activity_registrations.guest_data->>'name' ILIKE " + arg() + " or activity_registrations.guest_data->>'email' ILIKE " + arg() + ")"
	}
	if filters.Status != nil {
		statement += ` and "activity_registrations"."status" ilike ` + arg()
	}
	for _, filter := range []struct {
		name  string
		value *string
	}{{"university_id", filters.UniversityID}, {"province_id", filters.ProvinceID}, {"intake_year", filters.IntakeYear}} {
		if filter.value != nil {
			statement += " and " + quote("profiles."+filter.name) + " = " + arg()
		}
	}
	if count {
		return `select count(*) as "total" from (` + statement + `) as "subQuery"`
	}
	if !count {
		direction := "desc"
		if filters.Ascending {
			direction = "asc"
		}
		statement += " order by " + quote(registrationSortColumns[filters.SortBy]) + " " + direction + ` nulls last, "activity_registrations"."id" ` + direction + " limit " + arg()
		if filters.Page != 1 {
			statement += " offset " + arg()
		}
	}
	return statement
}
