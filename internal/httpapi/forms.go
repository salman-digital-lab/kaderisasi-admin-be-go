package httpapi

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/form"
	"net/http"
)

func (s *Server) registerForms() {
	controller := "custom_forms_controller"
	service := form.Service{Pool: s.Pool}
	for _, action := range []string{"index", "getUnattachedForms"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			params := r.URL.Query()
			sql := "SELECT * FROM custom_forms WHERE true"
			args := []interface{}{}
			if action == "getUnattachedForms" {
				sql += " AND feature_id IS NULL"
			}
			if search := params.Get("search"); search != "" {
				args = append(args, "%"+search+"%")
				sql += fmt.Sprintf(" AND form_name ILIKE $%d", len(args))
			}
			if action == "index" {
				for _, key := range []string{"feature_type", "feature_id"} {
					if value := params.Get(key); value != "" {
						args = append(args, value)
						sql += fmt.Sprintf(" AND %s=$%d", key, len(args))
					}
				}
				if params.Has("is_active") {
					args = append(args, params.Get("is_active") == "true")
					sql += fmt.Sprintf(" AND is_active=$%d", len(args))
				}
			}
			page, size := pageParams(r, 10, 0)
			data, err := s.queries().Paginate(r.Context(), sql+" ORDER BY created_at DESC", args, page, size)
			if err != nil {
				legacyFailure(w, paginationError(r, err))
				return nil
			}
			msg := "GET_DATA_SUCCESS"
			if action == "getUnattachedForms" {
				msg = "GET_UNATTACHED_FORMS_SUCCESS"
			}
			for _, row := range data.Data {
				database.Timestamps(row, s.Config.Location, "created_at", "updated_at")
			}
			reply(w, 200, msg, data)
			return nil
		})
	}
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		row, err := s.queries().One(r.Context(), "SELECT * FROM custom_forms WHERE id=$1", pathID(r, "id"))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Fail(404, "CUSTOM_FORM_NOT_FOUND")
		}
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
		return nil
	})
	s.register(controller, "getByFeature", func(w http.ResponseWriter, r *http.Request) error {
		kind, id := r.URL.Query().Get("feature_type"), r.URL.Query().Get("feature_id")
		if kind == "" || id == "" {
			return domain.Fail(400, "FEATURE_TYPE_AND_ID_REQUIRED")
		}
		row, err := s.queries().One(r.Context(), "SELECT * FROM custom_forms WHERE feature_type=$1 AND feature_id=$2 AND is_active=true ORDER BY updated_at DESC,id DESC LIMIT 1", kind, id)
		if errors.Is(err, pgx.ErrNoRows) {
			message(w, 200, "CUSTOM_FORM_NOT_FOUND_FOR_FEATURE")
			return nil
		}
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		reply(w, 200, "GET_DATA_SUCCESS", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
		return nil
	})
	for _, action := range []string{"store", "update", "attachToClub", "detachFromClub"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			schema := map[string]string{"store": "customFormValidator", "update": "updateCustomFormValidator", "attachToClub": "attachCustomFormToClubValidator"}[action]
			data := database.Object{}
			if schema != "" {
				var ok bool
				data, ok = caughtValidationInput(w, r, schema)
				if !ok {
					return nil
				}
			}
			var row database.Object
			var err error
			switch action {
			case "store":
				row, err = service.Create(r.Context(), data)
			case "update":
				row, err = service.Update(r.Context(), pathID(r, "id"), data, false)
			case "attachToClub":
				change := database.Object{}
				change.Set("feature_type", "club_registration")
				change.Set("feature_id", data.ID("clubId"))
				row, err = service.Update(r.Context(), pathID(r, "id"), change, true)
			case "detachFromClub":
				data.Set("feature_id", nil)
				row, err = service.Update(r.Context(), pathID(r, "id"), data, false)
			}
			if err != nil {
				var d *domain.Error
				if errors.As(err, &d) {
					return err
				}
				legacyFailure(w, paginationError(r, err))
				return nil
			}
			status := 200
			if action == "store" {
				status = 201
				database.CreatedModel(row, form.Canonical(data))
			}
			msg := map[string]string{"store": "CUSTOM_FORM_CREATED_SUCCESS", "update": "CUSTOM_FORM_UPDATED_SUCCESS", "attachToClub": "FORM_ATTACHED_TO_CLUB_SUCCESS", "detachFromClub": "FORM_DETACHED_FROM_CLUB_SUCCESS"}[action]
			reply(w, status, msg, database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
			return nil
		})
	}
	for _, action := range []string{"destroy", "toggleActive"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			ctx := r.Context()
			id := pathID(r, "id")
			row, err := s.queries().One(ctx, "SELECT * FROM custom_forms WHERE id=$1", id)
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.Fail(404, "CUSTOM_FORM_NOT_FOUND")
			}
			if err != nil {
				return err
			}
			if clubID := form.ClubID(row); clubID != 0 && (action == "destroy" || row.Bool("is_active")) {
				club, err := s.queries().One(ctx, "SELECT is_registration_open FROM clubs WHERE id=$1", clubID)
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
				if club.Bool("is_registration_open") {
					return domain.Fail(400, "CLOSE_REGISTRATION_BEFORE_FORM_CHANGE")
				}
			}
			if action == "destroy" {
				if _, err = s.queries().Delete(ctx, "custom_forms", id); err != nil {
					return err
				}
				message(w, 200, "CUSTOM_FORM_DELETED_SUCCESS")
			} else {
				change := database.Object{}
				change.Set("is_active", !row.Bool("is_active"))
				row, err = s.queries().Update(ctx, "custom_forms", id, change)
				if err != nil {
					return err
				}
				reply(w, 200, "CUSTOM_FORM_STATUS_UPDATED", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
			}
			return nil
		})
	}
	for _, target := range []struct{ action, table, kind, msg string }{{"getAvailableActivities", "activities", "activity_registration", "GET_AVAILABLE_ACTIVITIES_SUCCESS"}, {"getAvailableClubs", "clubs", "club_registration", "GET_AVAILABLE_CLUBS_SUCCESS"}} {
		s.register(controller, target.action, func(w http.ResponseWriter, r *http.Request) error {
			query := "SELECT id,name FROM " + target.table + " WHERE id NOT IN (SELECT feature_id FROM custom_forms WHERE feature_type=$1 AND feature_id IS NOT NULL"
			args := []interface{}{target.kind}
			if current := r.URL.Query().Get("current_form_id"); current != "" {
				query += " AND id<>$2"
				args = append(args, current)
			}
			query += ") ORDER BY name ASC"
			rows, err := s.queries().All(r.Context(), query, args...)
			if err != nil {
				legacyFailure(w, paginationError(r, err))
				return nil
			}
			reply(w, 200, target.msg, rows)
			return nil
		})
	}
	for _, action := range []string{"attachToActivity", "detachFromActivity"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			change := database.Object{}
			if action == "attachToActivity" {
				body := requestData(r)
				if body.ID("activityId") == 0 {
					return domain.Fail(400, "ACTIVITY_ID_REQUIRED")
				}
				change.Set("feature_id", body.ID("activityId"))
				change.Set("feature_type", "activity_registration")
			} else {
				change.Set("feature_id", nil)
			}
			row, err := s.queries().One(r.Context(), "SELECT * FROM custom_forms WHERE id=$1", pathID(r, "id"))
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.Fail(404, "CUSTOM_FORM_NOT_FOUND")
			}
			if err != nil {
				return err
			}
			if action == "attachToActivity" && row.ID("feature_id") != 0 {
				return domain.Fail(400, "FORM_ALREADY_ATTACHED")
			}
			row, err = s.queries().Update(r.Context(), "custom_forms", pathID(r, "id"), change)
			if err != nil {
				legacyFailure(w, paginationError(r, err))
				return nil
			}
			msg := "FORM_ATTACHED_TO_ACTIVITY_SUCCESS"
			if action == "detachFromActivity" {
				msg = "FORM_DETACHED_FROM_ACTIVITY_SUCCESS"
			}
			reply(w, 200, msg, database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
			return nil
		})
	}
}
