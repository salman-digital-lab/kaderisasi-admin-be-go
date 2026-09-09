package httpapi

import (
	"kaderisasi/admin/internal/activity"
	"net/http"
)

func (s *Server) listRegistrations(w http.ResponseWriter, r *http.Request) error {
	params := r.URL.Query()
	optional := func(key string) *string {
		value := params.Get(key)
		if value == "" {
			return nil
		}
		return &value
	}
	page, size := pageParams(r, 10, 0)
	filters := activity.RegistrationFilters{Search: optional("search"), Status: optional("status"), UniversityID: optional("university_id"), ProvinceID: optional("province_id"), IntakeYear: optional("intake_year"), SortBy: params.Get("sort_by"), Ascending: params.Get("sort_order") == "asc", Page: page, Size: size}
	result, err := (activity.Service{Pool: s.Pool}).ListRegistrations(r.Context(), r.PathValue("id"), filters)
	if err != nil {
		return registrationFailure(w, err)
	}
	plural(w, "GET_DATA_SUCCESS", result)
	return nil
}
