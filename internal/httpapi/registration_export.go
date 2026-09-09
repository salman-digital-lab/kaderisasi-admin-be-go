package httpapi

import (
	"fmt"
	"kaderisasi/admin/internal/activity"
	"net/http"
)

func sendWorkbook(w http.ResponseWriter, filename string, body []byte) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(200)
	_, _ = w.Write(body)
}
func (s *Server) exportRegistrations(w http.ResponseWriter, r *http.Request) error {
	document, err := (activity.Service{Pool: s.Pool}).ExportRegistrations(r.Context(), r.PathValue("id"))
	if err != nil {
		exportFailure(w, err)
		return nil
	}
	sendWorkbook(w, document.Filename, document.Body)
	return nil
}
