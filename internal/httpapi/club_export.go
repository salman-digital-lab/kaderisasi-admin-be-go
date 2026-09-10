package httpapi

import (
	"kaderisasi/admin/internal/club"
	"net/http"
)

func (s *Server) exportClubRegistrations(w http.ResponseWriter, r *http.Request) error {
	result, err := (club.Service{Pool: s.Pool, Location: s.Config.Location}).ExportRegistrations(r.Context(), r.PathValue("id"))
	if err != nil {
		legacyFailure(w, err)
		return nil
	}
	sendWorkbook(w, result.Filename, result.Body)
	return nil
}
