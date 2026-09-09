package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/activity"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"net/http"
)

func nestedObject(data database.Object, key string) database.Object {
	out := database.Object{}
	_ = json.Unmarshal(data[key], &out)
	if out == nil {
		out = database.Object{}
	}
	return out
}
func (s *Server) validateTemplateAssignment(w http.ResponseWriter, r *http.Request, id int32) (bool, error) {
	if !auth.ForRole(actor(r).RoleCode, true).Allows("certificate.template.manage") {
		return false, domain.Fail(403, "FORBIDDEN")
	}
	if id == 0 {
		return true, nil
	}
	template, err := s.queries().One(r.Context(), "SELECT * FROM certificate_templates WHERE id=$1", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, domain.Fail(422, "CERTIFICATE_TEMPLATE_NOT_FOUND")
	}
	if err != nil {
		return false, err
	}
	readiness := certificate.CheckReadiness(template)
	if template.String("lifecycle_status") != "published" || !readiness.Ready {
		write(w, 422, struct {
			Message string   `json:"message"`
			Errors  []string `json:"errors"`
		}{"CERTIFICATE_TEMPLATE_NOT_READY", readiness.Errors})
		return false, nil
	}
	return true, nil
}
func (s *Server) registerActivities() {
	s.registerActivityReads()
	s.register("activities_controller", "store", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := input(w, r, "activityValidator")
		if !ok {
			return nil
		}
		config := nestedObject(data, "additional_config")
		templateID := data.ID("certificate_template_id")
		if templateID == 0 {
			templateID = config.ID("certificate_template_id")
		}
		if templateID != 0 {
			ok, err := s.validateTemplateAssignment(w, r, templateID)
			if err != nil || !ok {
				return err
			}
		}
		if templateID != 0 {
			data.Set("certificate_template_id", templateID)
			config.Set("certificate_template_id", templateID)
		} else {
			data.Set("certificate_template_id", nil)
			config.Set("certificate_template_id", nil)
		}
		if data.Has("additional_config") {
			data.Set("additional_config", config)
		}
		if !data.Has("club_id") {
			data.Set("club_id", nil)
		}
		base := activity.Slug(data.String("name"))
		slug := base
		for n := 2; ; n++ {
			_, err := s.queries().One(r.Context(), "SELECT id FROM activities WHERE slug=$1", slug)
			if errors.Is(err, pgx.ErrNoRows) {
				break
			}
			if err != nil {
				return err
			}
			slug = fmt.Sprintf("%s-%d", base, n)
		}
		data.Set("slug", slug)
		row, err := s.queries().Insert(r.Context(), "activities", data)
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		for key := range row {
			if !data.Has(key) && key != "id" && key != "created_at" && key != "updated_at" {
				delete(row, key)
			}
		}
		if data.Has("is_published") {
			row["is_published"] = data["is_published"]
		}
		reply(w, 200, "CREATE_DATA_SUCCESS", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
		return nil
	})
	s.register("activities_controller", "update", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := input(w, r, "updateActivityValidator")
		if !ok {
			return nil
		}
		id := pathID(r, "id")
		old, err := s.queries().One(r.Context(), "SELECT * FROM activities WHERE id=$1", id)
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		config := nestedObject(data, "additional_config")
		provided := data.Has("certificate_template_id") || config.Has("certificate_template_id")
		requested := config.ID("certificate_template_id")
		if data.Has("certificate_template_id") {
			requested = data.ID("certificate_template_id")
		}
		if provided && requested != old.ID("certificate_template_id") {
			ok, err := s.validateTemplateAssignment(w, r, requested)
			if err != nil || !ok {
				return err
			}
		}
		merged := nestedObject(old, "additional_config")
		images := merged["images"]
		if len(images) == 0 || string(images) == "null" {
			images = json.RawMessage("[]")
		}
		for key, value := range config {
			merged[key] = value
		}
		merged["images"] = images
		if provided {
			if requested == 0 {
				data.Set("certificate_template_id", nil)
				merged.Set("certificate_template_id", nil)
			} else {
				data.Set("certificate_template_id", requested)
				merged.Set("certificate_template_id", requested)
			}
		}
		data.Set("additional_config", merged)
		row, err := s.queries().Update(r.Context(), "activities", id, data)
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		for _, key := range []string{"activity_start", "activity_end", "registration_start", "registration_end", "selection_start", "selection_end"} {
			if !data.Has(key) || data.Null(key) {
				delete(row, key)
			}
		}
		if data.Has("is_published") {
			row["is_published"] = data["is_published"]
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
		return nil
	})
	// The legacy delete action is retained even though it is currently not routed.
	s.register("activities_controller", "delete", func(w http.ResponseWriter, r *http.Request) error {
		removed, err := s.queries().Delete(r.Context(), "activities", pathID(r, "id"))
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		if !removed {
			message(w, 200, "ACTIVITY_NOT_FOUND")
		} else {
			message(w, 200, "DELETE_DATA_SUCCESS")
		}
		return nil
	})
	s.registerActivityMedia()
}
