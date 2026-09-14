package httpapi

import (
	"context"
	"encoding/json"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"slices"
	"strconv"
)

func (s *Server) featureDeletion(remove func(context.Context, string, string) error) Handler {
	return func(w http.ResponseWriter, r *http.Request) error {
		user := actor(r)
		codes := auth.RoleCodes(user.RoleCode, user.AdditionalRoleCodes)
		if !user.IsActive || (!slices.Contains(codes, "super_admin") && !slices.Contains(codes, "admin")) {
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
