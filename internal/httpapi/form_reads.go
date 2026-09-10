package httpapi

import (
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/form"
	"net/http"
)

func (s *Server) registerFormReads() {
	controller := "custom_forms_controller"
	service := form.Service{Pool: s.Pool}
	for _, action := range []string{"index", "getUnattachedForms"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			params := r.URL.Query()
			optional := func(key string) *string {
				value := params.Get(key)
				if value == "" {
					return nil
				}
				return &value
			}
			page, size := pageParams(r, 10, 0)
			filters := form.Filters{Search: optional("search"), Unattached: action == "getUnattachedForms", Page: page, Size: size}
			if !filters.Unattached {
				filters.FeatureType = optional("feature_type")
				filters.FeatureID = optional("feature_id")
				if params.Has("is_active") {
					active := params.Get("is_active") == "true"
					filters.IsActive = &active
				}
			}
			result, err := service.List(r.Context(), filters, s.Config.Location)
			if err != nil {
				legacyFailure(w, paginationError(r, err))
				return nil
			}
			success := "GET_DATA_SUCCESS"
			if filters.Unattached {
				success = "GET_UNATTACHED_FORMS_SUCCESS"
			}
			reply(w, 200, success, result)
			return nil
		})
	}
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.Show(r.Context(), r.PathValue("id"), s.Config.Location)
		if err != nil {
			return clubFailure(w, err)
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "getByFeature", func(w http.ResponseWriter, r *http.Request) error {
		kind, id := r.URL.Query().Get("feature_type"), r.URL.Query().Get("feature_id")
		if kind == "" || id == "" {
			return domain.Fail(400, "FEATURE_TYPE_AND_ID_REQUIRED")
		}
		result, err := service.ByFeature(r.Context(), kind, id, s.Config.Location)
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		if result == nil {
			message(w, 200, "CUSTOM_FORM_NOT_FOUND_FOR_FEATURE")
			return nil
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	for _, clubs := range []bool{false, true} {
		action, success := "getAvailableActivities", "GET_AVAILABLE_ACTIVITIES_SUCCESS"
		if clubs {
			action, success = "getAvailableClubs", "GET_AVAILABLE_CLUBS_SUCCESS"
		}
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			var current *string
			if value := r.URL.Query().Get("current_form_id"); value != "" {
				current = &value
			}
			result, err := service.Available(r.Context(), clubs, current)
			if err != nil {
				legacyFailure(w, err)
				return nil
			}
			reply(w, 200, success, result)
			return nil
		})
	}
}
