package httpapi

import (
	"errors"
	"kaderisasi/admin/internal/activity"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"net/http"
)

func (s *Server) registerActivities() {
	s.registerActivityReads()
	s.registerActivityMedia()
	service := activity.Service{Pool: s.Pool, Storage: s.Storage}
	for _, action := range []string{"store", "update"} {
		s.register("activities_controller", action, func(w http.ResponseWriter, r *http.Request) error {
			schema := "activityValidator"
			if action == "update" {
				schema = "updateActivityValidator"
			}
			data, ok := inputAs[activity.Input](w, r, schema)
			if !ok {
				return nil
			}
			canManage := auth.ForRole(actor(r).RoleCode, true).Allows("certificate.template.manage")
			if action == "store" {
				created, err := service.Create(r.Context(), data, canManage)
				if err != nil {
					var d *domain.Error
					if errors.As(err, &d) {
						return err
					}
					legacyFailure(w, err)
					return nil
				}
				reply(w, 200, "CREATE_DATA_SUCCESS", created)
			} else {
				updated, err := service.Update(r.Context(), pathID(r, "id"), data, canManage)
				if err != nil {
					var d *domain.Error
					if errors.As(err, &d) {
						return err
					}
					legacyFailure(w, err)
					return nil
				}
				reply(w, 200, "UPDATE_DATA_SUCCESS", updated)
			}
			return nil
		})
	}
	// Kept for source parity; this action has no route declaration.
	s.register("activities_controller", "delete", func(w http.ResponseWriter, r *http.Request) error {
		removed, err := dbgen.New(s.Pool).DeleteActivity(r.Context(), pathID(r, "id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		if removed == 0 {
			message(w, 200, "ACTIVITY_NOT_FOUND")
		} else {
			message(w, 200, "DELETE_DATA_SUCCESS")
		}
		return nil
	})
}
