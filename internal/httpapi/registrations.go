package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/activity"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"strings"
	"time"
)

func affected(w http.ResponseWriter, count int64) {
	write(w, 200, struct {
		Messages string  `json:"messages"`
		Affected []int64 `json:"affected_rows"`
	}{"UPDATE_DATA_SUCCESS", []int64{count}})
}
func (s *Server) registerRegistrations() {
	controller := "activity_registrations_controller"
	s.register(controller, "export", s.exportRegistrations)
	s.register(controller, "store", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := input(w, r, "storeActivityRegistration")
		if !ok {
			return nil
		}
		profile, err := s.queries().One(r.Context(), "SELECT * FROM profiles WHERE id=$1", data.ID("user_id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		activity, err := s.queries().One(r.Context(), "SELECT * FROM activities WHERE id=$1", pathID(r, "id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		count, err := s.queries().Count(r.Context(), "SELECT id FROM activity_registrations WHERE user_id=$1 AND activity_id=$2", profile.ID("user_id"), activity.ID("id"))
		if err != nil {
			return err
		}
		if count > 0 {
			return domain.Fail(409, "ALREADY_REGISTERED")
		}
		if profile.Number("level") < activity.Number("minimum_level") {
			return domain.Fail(403, "UNMATCHED_LEVEL")
		}
		data.Set("user_id", profile.ID("user_id"))
		data.Set("activity_id", activity.ID("id"))
		data.Set("status", "TERDAFTAR")
		row, err := s.queries().Insert(r.Context(), "activity_registrations", data)
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		delete(row, "guest_data")
		plural(w, "CREATE_DATA_SUCCESS", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		row, err := s.queries().One(r.Context(), "SELECT * FROM activity_registrations WHERE id=$1", pathID(r, "id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		plural(w, "GET_DATA_SUCCESS", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
		return nil
	})
	s.register(controller, "getActivityByUserId", func(w http.ResponseWriter, r *http.Request) error {
		rows, err := s.queries().All(r.Context(), "SELECT ar.*,row_to_json(a) AS activity FROM activity_registrations ar LEFT JOIN activities a ON a.id=ar.activity_id WHERE ar.user_id=$1", pathID(r, "id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		for _, row := range rows {
			database.Timestamps(row, s.Config.Location, "created_at", "updated_at")
			if !row.Null("activity") {
				a := nestedObject(row, "activity")
				row.Set("activity", database.Timestamps(a, s.Config.Location, "created_at", "updated_at"))
			}
		}
		reply(w, 200, "GET_DATA_SUCCESS", rows)
		return nil
	})
	s.register(controller, "statistics", func(w http.ResponseWriter, r *http.Request) error {
		rows, err := s.queries().All(r.Context(), "SELECT status,count(*)::int AS count FROM activity_registrations WHERE activity_id=$1 GROUP BY status", pathID(r, "id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		byStatus := map[string]int32{}
		total := int32(0)
		for _, row := range rows {
			count := row.ID("count")
			byStatus[row.String("status")] = count
			total += count
		}
		plural(w, "GET_DATA_SUCCESS", struct {
			Total    int32            `json:"total"`
			ByStatus map[string]int32 `json:"by_status"`
		}{total, byStatus})
		return nil
	})
	s.register(controller, "index", s.listRegistrations)
	s.register(controller, "updateStatus", s.updateRegistrations)
	s.register(controller, "updateStatusByListOfEmail", s.updateRegistrations)
	s.register(controller, "updateStatusBulk", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := input(w, r, "bulkUpdateActivityRegistrations")
		if !ok {
			return nil
		}
		query := "UPDATE activity_registrations SET status=$1 WHERE activity_id=$2"
		args := []interface{}{data.String("new_status"), pathID(r, "id")}
		// Preserve the original name filter, including its database error: the legacy
		// endpoint applies it to registrations, whose schema has no name column.
		for _, pair := range [][2]string{{"name", "name"}, {"current_status", "status"}} {
			if v := data.String(pair[0]); v != "" {
				args = append(args, v)
				query += fmt.Sprintf(" AND %s=$%d", pair[1], len(args))
			}
		}
		tag, err := s.Pool.Exec(r.Context(), query, args...)
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		affected(w, tag.RowsAffected())
		return nil
	})
	s.register(controller, "delete", func(w http.ResponseWriter, r *http.Request) error {
		id := pathID(r, "id")
		_, err := s.queries().One(r.Context(), "SELECT id FROM activity_registrations WHERE id=$1", id)
		if errors.Is(err, pgx.ErrNoRows) {
			message(w, 200, "REGISTRATION_NOT_FOUND")
			return nil
		}
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		count, err := s.queries().Count(r.Context(), "SELECT id FROM issued_certificates WHERE registration_id=$1", id)
		if err != nil {
			return err
		}
		if count > 0 {
			return domain.Fail(409, "CERTIFICATE_REGISTRATION_HAS_ISSUED_CERTIFICATE")
		}
		if _, err = s.queries().Delete(r.Context(), "activity_registrations", id); err != nil {
			legacyFailure(w, err)
			return nil
		}
		message(w, 200, "DELETE_DATA_SUCCESS")
		return nil
	})
}
func (s *Server) listRegistrations(w http.ResponseWriter, r *http.Request) error {
	activity, err := s.queries().One(r.Context(), "SELECT * FROM activities WHERE id=$1", pathID(r, "id"))
	if err != nil {
		legacyFailure(w, err)
		return nil
	}
	var required []struct {
		Name string `json:"name"`
	}
	config := nestedObject(activity, "additional_config")
	if err = json.Unmarshal(config["mandatory_profile_data"], &required); err != nil {
		legacyFailure(w, fmt.Errorf("Cannot read properties of undefined (reading 'map')"))
		return nil
	}
	fields := []string{"ar.id", "u.id AS user_id", "COALESCE(u.email,ar.guest_data->>'email') AS email", "COALESCE(p.name,ar.guest_data->>'name') AS name", "p.level", "p.university_id", "p.province_id", "p.intake_year", "p.major", "COALESCE(p.gender,ar.guest_data->>'gender') AS gender", "COALESCE(p.whatsapp,ar.guest_data->>'whatsapp') AS whatsapp", "p.instagram", "p.line", "p.personal_id", "p.education_history", "ar.guest_data"}
	for _, field := range required {
		fields = append(fields, "p."+pgx.Identifier{field.Name}.Sanitize())
	}
	fields = append(fields, "ar.status", "ar.created_at")
	sql := "SELECT " + strings.Join(fields, ",") + " FROM activity_registrations ar LEFT JOIN public_users u ON u.id=ar.user_id LEFT JOIN profiles p ON p.user_id=ar.user_id WHERE ar.activity_id=$1"
	args := []interface{}{pathID(r, "id")}
	params := r.URL.Query()
	if search := params.Get("search"); search != "" {
		args = append(args, "%"+search+"%")
		sql += fmt.Sprintf(" AND (p.name ILIKE $%[1]d OR u.email ILIKE $%[1]d OR ar.guest_data->>'name' ILIKE $%[1]d OR ar.guest_data->>'email' ILIKE $%[1]d)", len(args))
	}
	if status := params.Get("status"); status != "" {
		args = append(args, "%"+status+"%")
		sql += fmt.Sprintf(" AND ar.status ILIKE $%d", len(args))
	}
	for _, key := range []string{"university_id", "province_id", "intake_year"} {
		if value := params.Get(key); value != "" {
			args = append(args, value)
			sql += fmt.Sprintf(" AND p.%s=$%d", key, len(args))
		}
	}
	columns := map[string]string{"created_at": "ar.created_at", "name": "p.name", "email": "u.email", "status": "ar.status", "level": "p.level", "university_id": "p.university_id", "province_id": "p.province_id", "intake_year": "p.intake_year", "major": "p.major", "whatsapp": "p.whatsapp"}
	column := columns[params.Get("sort_by")]
	if column == "" {
		column = "ar.created_at"
	}
	direction := "DESC"
	if params.Get("sort_order") == "asc" {
		direction = "ASC"
	}
	sql += " ORDER BY " + column + " " + direction + " NULLS LAST,ar.id " + direction
	page, size := pageParams(r, 10, 0)
	result, err := s.queries().Paginate(r.Context(), sql, args, page, size)
	if err != nil {
		legacyFailure(w, err)
		return nil
	}
	for _, row := range result.Data {
		database.Timestamps(row, time.UTC, "created_at")
	}
	plural(w, "GET_DATA_SUCCESS", result.RawPage())
	return nil
}
func (s *Server) updateRegistrations(w http.ResponseWriter, r *http.Request) error {
	byEmail := r.PathValue("id") != ""
	schema := "updateActivityRegistrations"
	if byEmail {
		schema = "updateActivityRegistrationsByEmail"
	}
	data, ok := input(w, r, schema)
	if !ok {
		return nil
	}
	ctx := r.Context()
	var a database.Object
	var err error
	ids := []int32{}
	if byEmail {
		a, err = s.queries().One(ctx, "SELECT * FROM activities WHERE id=$1", pathID(r, "id"))
	} else {
		_ = json.Unmarshal(data["registrations_id"], &ids)
		first := int32(0)
		if len(ids) > 0 {
			first = ids[0]
		}
		a, err = s.queries().One(ctx, "SELECT a.* FROM activities a JOIN activity_registrations ar ON a.id=ar.activity_id WHERE ar.id=$1", first)
	}
	if err != nil {
		legacyFailure(w, err)
		return nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	users := []int32{}
	if byEmail {
		emails := []string{}
		_ = json.Unmarshal(data["emails"], &emails)
		rows, err := q.All(ctx, "SELECT id FROM public_users WHERE email=ANY($1::text[])", emails)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return domain.Fail(404, "NO_USERS_FOUND")
		}
		for _, row := range rows {
			users = append(users, row.ID("id"))
		}
		registrations, err := q.All(ctx, "SELECT id FROM activity_registrations WHERE activity_id=$1 AND user_id=ANY($2::int[])", a.ID("id"), users)
		if err != nil {
			return err
		}
		if len(registrations) == 0 {
			return domain.Fail(404, "NO_REGISTRATIONS_FOUND")
		}
		for _, row := range registrations {
			ids = append(ids, row.ID("id"))
		}
	} else {
		rows, err := q.All(ctx, "SELECT user_id FROM activity_registrations WHERE id=ANY($1::int[])", ids)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if !row.Null("user_id") {
				users = append(users, row.ID("user_id"))
			}
		}
	}
	if err = activity.Upgrade(ctx, tx, a, users, data.String("status")); err != nil {
		legacyFailure(w, err)
		return nil
	}
	tag, err := tx.Exec(ctx, "UPDATE activity_registrations SET status=$1 WHERE id=ANY($2::int[])", data.String("status"), ids)
	if err != nil {
		legacyFailure(w, err)
		return nil
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	affected(w, tag.RowsAffected())
	return nil
}
