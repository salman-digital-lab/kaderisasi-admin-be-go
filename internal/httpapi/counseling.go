package httpapi

import (
	"errors"
	"kaderisasi/admin/internal/counseling"
	"kaderisasi/admin/internal/domain"
	"net/http"
)

func (s *Server) registerCounseling() {
	controller := "ruang_curhats_controller"
	service := counseling.Service{Pool: s.Pool, Location: s.Config.Location}
	s.register(controller, "counselors", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.CounselorOptions(r.Context())
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		params := r.URL.Query()
		optional := func(key string) *string {
			value := params.Get(key)
			if value == "" {
				return nil
			}
			return &value
		}
		page, size := pageParams(r, 10, 0)
		result, err := service.List(r.Context(), counseling.Filters{Status: optional("status"), Name: optional("name"), Gender: optional("gender"), AdminName: optional("admin_display_name"), Page: page, Size: size})
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		plural(w, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.Show(r.Context(), r.PathValue("id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "update", func(w http.ResponseWriter, r *http.Request) error {
		input, ok := inputAs[counseling.Input](w, r, "UpdateRuangCurhatValidator")
		if !ok {
			return nil
		}
		result, err := service.Update(r.Context(), r.PathValue("id"), input)
		if err != nil {
			var failure *domain.Error
			if errors.As(err, &failure) {
				return err
			}
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", result)
		return nil
	})
}
