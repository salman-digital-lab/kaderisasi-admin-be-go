package httpapi

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"net/http"
	"slices"
	"strings"
)

// Legacy controllers expose the prepared statement in pagination failures.
// Render that public diagnostic separately from the sqlc execution query: its
// aliases, relation loading and bind positions are implementation details.
func paginationError(r *http.Request, err error) error {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || !slices.Contains([]string{"2201W", "22003"}, pg.Code) {
		return err
	}
	params := r.URL.Query()
	bind := 0
	argument := func() string { bind++; return fmt.Sprintf("$%d", bind) }
	conditions := []string{}
	add := func(column, op string) { conditions = append(conditions, `"`+column+`" `+op+` `+argument()) }
	columns, table, order := "*", "", ""
	switch r.URL.Path {
	case "/v2/admin-users":
		table, order = "admin_users", `"created_at" desc`
		if strings.TrimSpace(params.Get("search")) != "" {
			conditions = append(conditions, `("email" ilike `+argument()+` or "display_name" ilike `+argument()+`)`)
		}
	case "/v2/universities":
		table, order = "universities", `"name" asc`
		add("name", "ilike")
	case "/v2/profiles":
		table, order = "profiles", `"name" asc`
		group := []string{}
		if params.Get("search") != "" {
			group = append(group, `"name" ilike `+argument()+` or exists (select * from "public_users" where ("email" ilike `+argument()+` or "member_id" ilike `+argument()+`) and ("public_users"."id" = "profiles"."user_id"))`)
		}
		if params.Get("member_id") != "" {
			group = append(group, `exists (select * from "public_users" where ("member_id" = `+argument()+`) and ("public_users"."id" = "profiles"."user_id"))`)
		}
		if params.Get("education_institution") != "" {
			group = append(group, `EXISTS (
                SELECT 1 FROM jsonb_array_elements(
  CASE
    WHEN jsonb_typeof(education_history) = 'array' THEN education_history
    ELSE '[]'::jsonb
  END
) AS edu
                WHERE edu->>'institution' ILIKE `+argument()+`
              )`)
		}
		if len(group) > 0 {
			conditions = append(conditions, "("+strings.Join(group, " and ")+")")
		}
		if params.Get("badge") != "" {
			conditions = append(conditions, `EXISTS (
          SELECT 1 FROM jsonb_array_elements_text(
  CASE
    WHEN jsonb_typeof(badges) = 'array' THEN badges
    WHEN jsonb_typeof(badges) = 'string' THEN jsonb_build_array(badges #>> '{}')
    ELSE '[]'::jsonb
  END
) badge
          WHERE badge ILIKE `+argument()+`
        )`)
		}
	case "/v2/activities":
		table, order = "activities", `"is_published" desc, "created_at" desc`
		columns = `"id", "name", "activity_start", "activity_end", "registration_start", "registration_end", "selection_start", "selection_end", "activity_type", "activity_category", "club_id", "is_published", "is_registration_open"`
		for _, filter := range [][2]string{{"category", "activity_category"}, {"minimum_level", "minimum_level"}, {"activity_type", "activity_type"}, {"is_published", "is_published"}, {"club_id", "club_id"}} {
			if params.Get(filter[0]) != "" {
				add(filter[1], "=")
			}
		}
		add("name", "ilike")
	case "/v2/clubs":
		table, order = "clubs", `"is_show" desc, "created_at" desc`
		columns = `"id", "name", "club_type", "description", "short_description", "logo", "created_at", "updated_at", "start_period", "end_period", "is_show", "is_registration_open", "registration_end_date"`
		add("name", "ilike")
		if slices.Contains([]string{"UNIT", "CLUB_KEPROFESIAN", "CLUB_BAHASA", "AVISMAN_REGIONAL"}, params.Get("club_type")) {
			add("club_type", "=")
		}
		if value := params.Get("visibility"); value == "published" || value == "draft" {
			add("is_show", "=")
		}
		if value := params.Get("registration"); value == "open" || value == "closed" {
			add("is_registration_open", "=")
		}
	case "/v2/custom-forms/unattached":
		table, order = "custom_forms", `"created_at" desc`
		conditions = append(conditions, `"feature_id" is null`)
		if params.Get("search") != "" {
			add("form_name", "ilike")
		}
	case "/v2/custom-forms":
		table, order = "custom_forms", `"created_at" desc`
		for _, filter := range [][3]string{{"search", "form_name", "ilike"}, {"feature_type", "feature_type", "="}, {"feature_id", "feature_id", "="}} {
			if params.Get(filter[0]) != "" {
				add(filter[1], filter[2])
			}
		}
		if params.Has("is_active") {
			add("is_active", "=")
		}
	default:
		return err
	}
	statement := `select ` + columns + ` from "` + table + `"`
	if len(conditions) > 0 {
		statement += " where " + strings.Join(conditions, " and ")
	}
	statement += " order by " + order + " limit " + argument()
	if queryNumber(r, "page", 1) != 1 {
		statement += " offset " + argument()
	}
	return errors.New(statement + " - " + pg.Message)
}
