package httpapi

import (
	"encoding/json"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/export"
	"kaderisasi/admin/internal/member"
	"net/http"
)

func profileFilters(r *http.Request) dbgen.CountProfilesFilteredParams {
	params := r.URL.Query()
	return dbgen.CountProfilesFilteredParams{Search: params.Get("search"), MemberNumber: params.Get("member_id"), Institution: params.Get("education_institution"), Badge: params.Get("badge")}
}

func (s *Server) registerMemberExport() {
	service := member.Service{Pool: s.Pool}
	s.register("profiles_controller", "exportPreview", func(w http.ResponseWriter, r *http.Request) error {
		if !auth.ForUser(actor(r)).IsSuperAdmin {
			return domain.Fail(403, "FORBIDDEN")
		}
		preview, err := service.PreviewExport(r.Context(), profileFilters(r))
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", preview)
		return nil
	})
	s.register("profiles_controller", "export", func(w http.ResponseWriter, r *http.Request) error {
		if !auth.ForUser(actor(r)).IsSuperAdmin {
			return domain.Fail(403, "FORBIDDEN")
		}
		var request struct {
			Columns []string `json:"columns"`
			Format  string   `json:"format"`
		}
		data, err := json.Marshal(requestData(r))
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &request); err != nil {
			return domain.Fail(422, "INVALID_MEMBER_EXPORT")
		}
		document, err := service.Export(r.Context(), profileFilters(r), request.Columns, request.Format)
		if err != nil {
			return err
		}
		body, err := export.MemberFile(document.Headers, document.Rows, request.Format)
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "no-store")
		if request.Format == "xlsx" {
			sendWorkbook(w, "anggota.xlsx", body)
			return nil
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="anggota.csv"`)
		_, err = w.Write(body)
		return err
	})
}
