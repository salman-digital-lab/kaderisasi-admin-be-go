package httpapi

import (
	"context"
	"encoding/json"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"strconv"
)

func (s *Server) featureDeletion(remove func(context.Context, string, string) error) Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		role := actor(r).RoleCode
		if role == nil || (*role != "super_admin" && *role != "admin") {
			return domain.Fail(403, "FORBIDDEN")
		}
		identifier := pathID(r, "id")
		if id, err := strconv.ParseInt(identifier, 10, 32); err != nil || id < 1 {
			return domain.Fail(404, "DATA_NOT_FOUND")
		}
		var confirmation string
		if err := json.Unmarshal(requestData(r)["confirmation"], &confirmation); err != nil || confirmation == "" {
			return domain.Fail(422, "DELETE_CONFIRMATION_MISMATCH")
		}
		if err := remove(r.Context(), identifier, confirmation); err != nil {
			return err
		}
		message(w, 200, "DELETE_DATA_SUCCESS")
		return nil
	}
}
