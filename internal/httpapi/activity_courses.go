package httpapi

import (
	"kaderisasi/admin/internal/activity"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) registerActivityCourses() {
	service := activity.Service{Pool: s.Pool}
	s.register("activity_courses_controller", "options", func(w http.ResponseWriter, r *http.Request) error {
		page, size := coursePagination(r)
		result, err := service.CourseOptions(r.Context(), strings.TrimSpace(r.URL.Query().Get("search")), page, size)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register("activity_courses_controller", "show", func(w http.ResponseWriter, r *http.Request) error {
		user := actor(r)
		permissions := auth.ForRole(user.RoleCode, user.IsActive)
		if !permissions.Allows("activities.read") && !permissions.Allows("activity_registrations.read") {
			return domain.Fail(403, "FORBIDDEN")
		}
		id, err := activityCourseID(r)
		if err != nil {
			return err
		}
		result, err := service.LinkedCourses(r.Context(), id)
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "private, no-store")
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register("activity_courses_controller", "update", func(w http.ResponseWriter, r *http.Request) error {
		id, err := activityCourseID(r)
		if err != nil {
			return err
		}
		input, err := courseInput[activity.CourseLinksInput](r)
		if err != nil {
			return domain.Fail(422, "INVALID_ACTIVITY_COURSES")
		}
		result, err := service.SaveLinkedCourses(r.Context(), id, input)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", result)
		return nil
	})
}

func activityCourseID(r *http.Request) (int32, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil || id < 1 {
		return 0, domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	return int32(id), nil
}
