package httpapi

import (
	"errors"
	"kaderisasi/admin/internal/activity"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"net/http"
)

func affected(w http.ResponseWriter, count int64) {
	write(w, 200, struct {
		Messages string  `json:"messages"`
		Affected []int64 `json:"affected_rows"`
	}{"UPDATE_DATA_SUCCESS", []int64{count}})
}
func registrationFailure(w http.ResponseWriter, err error) error {
	var business *domain.Error
	if errors.As(err, &business) {
		return err
	}
	legacyFailure(w, err)
	return nil
}
func (s *Server) registerRegistrations() {
	controller := "activity_registrations_controller"
	service := activity.Service{Pool: s.Pool}
	s.register(controller, "export", s.exportRegistrations)
	s.register(controller, "index", s.listRegistrations)
	s.register(controller, "store", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[activity.RegistrationInput](w, r, "storeActivityRegistration")
		if !ok {
			return nil
		}
		row, err := service.Register(r.Context(), r.PathValue("id"), data)
		if err != nil {
			return registrationFailure(w, err)
		}
		plural(w, "CREATE_DATA_SUCCESS", row)
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		row, err := dbgen.New(s.Pool).ActivityRegistrationByID(r.Context(), r.PathValue("id"))
		err = database.LegacyQueryError(err, `select * from "activity_registrations" where "id" = $1 limit $2`)
		if err != nil {
			return registrationFailure(w, err)
		}
		plural(w, "GET_DATA_SUCCESS", activity.RegistrationView(row))
		return nil
	})
	s.register(controller, "getActivityByUserId", func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		rows, err := dbgen.New(s.Pool).RegistrationsByUser(r.Context(), id)
		if err != nil {
			return registrationFailure(w, err)
		}
		result := make([]activity.UserRegistration, 0, len(rows))
		for _, row := range rows {
			relation, err := activity.FromRelation(row.Activity)
			if err != nil {
				return registrationFailure(w, err)
			}
			result = append(result, activity.UserRegistration{RegistrationResponse: activity.RegistrationView(row.ActivityRegistration), Activity: relation})
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "statistics", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.RegistrationStatistics(r.Context(), r.PathValue("id"))
		if err != nil {
			return registrationFailure(w, err)
		}
		plural(w, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "updateStatus", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[activity.RegistrationStatusInput](w, r, "updateActivityRegistrations")
		if !ok {
			return nil
		}
		count, err := service.ChangeRegistrationStatuses(r.Context(), data)
		if err != nil {
			return registrationFailure(w, err)
		}
		affected(w, count)
		return nil
	})
	s.register(controller, "updateStatusByListOfEmail", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[activity.RegistrationEmailStatusInput](w, r, "updateActivityRegistrationsByEmail")
		if !ok {
			return nil
		}
		count, err := service.ChangeRegistrationStatusesByEmail(r.Context(), r.PathValue("id"), data)
		if err != nil {
			return registrationFailure(w, err)
		}
		affected(w, count)
		return nil
	})
	s.register(controller, "updateStatusBulk", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[activity.RegistrationBulkStatusInput](w, r, "bulkUpdateActivityRegistrations")
		if !ok {
			return nil
		}
		count, err := service.ChangeRegistrationStatusesBulk(r.Context(), r.PathValue("id"), data)
		if err != nil {
			return registrationFailure(w, err)
		}
		affected(w, count)
		return nil
	})
	s.register(controller, "delete", func(w http.ResponseWriter, r *http.Request) error {
		removed, err := service.DeleteRegistration(r.Context(), r.PathValue("id"))
		if err != nil {
			return registrationFailure(w, err)
		}
		if removed {
			message(w, 200, "DELETE_DATA_SUCCESS")
		} else {
			message(w, 200, "REGISTRATION_NOT_FOUND")
		}
		return nil
	})
}
