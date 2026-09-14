package httpapi

import (
	"kaderisasi/admin/internal/activity"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/linkedcourse"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) registerLinkedCourses() {
	service := linkedcourse.Service{Pool: s.Pool}
	s.register("club_courses_controller", "options", func(w http.ResponseWriter, r *http.Request) error {
		page, size := coursePagination(r)
		result, err := (activity.Service{Pool: s.Pool}).CourseOptions(r.Context(), strings.TrimSpace(r.URL.Query().Get("search")), page, size)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register("club_courses_controller", "show", func(w http.ResponseWriter, r *http.Request) error {
		a := actor(r)
		p := auth.ForUser(a)
		if !p.Allows("clubs.read") && !p.Allows("club_registrations.read") {
			return domain.Fail(403, "FORBIDDEN")
		}
		scope, err := linkedCourseScope(r, "club")
		if err != nil {
			return err
		}
		result, err := service.Links(r.Context(), scope)
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "private, no-store")
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register("club_courses_controller", "update", func(w http.ResponseWriter, r *http.Request) error {
		scope, err := linkedCourseScope(r, "club")
		if err != nil {
			return err
		}
		in, err := courseInput[linkedcourse.LinksInput](r)
		if err != nil {
			return domain.Fail(422, "INVALID_CLUB_COURSES")
		}
		result, err := service.SaveClubLinks(r.Context(), scope.ID, in)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", result)
		return nil
	})
	for _, kind := range []string{"activity", "club"} {
		s.register(kind+"_course_progress_controller", "index", func(w http.ResponseWriter, r *http.Request) error {
			scope, err := linkedCourseScope(r, kind)
			if err != nil {
				return err
			}
			page, size := coursePagination(r)
			params := r.URL.Query()
			f := linkedcourse.Filters{Page: page, Size: size, Search: strings.TrimSpace(params.Get("search")), Status: params.Get("status"), Completion: params.Get("course_completion")}
			if raw := params.Get("course_id"); raw != "" {
				id, err := strconv.ParseInt(raw, 10, 32)
				if err != nil || id < 1 {
					return domain.Fail(422, "INVALID_COURSE_PROGRESS_FILTER")
				}
				f.CourseID = int32(id)
			}
			result, err := service.People(r.Context(), scope, f)
			if err != nil {
				return err
			}
			w.Header().Set("Cache-Control", "private, no-store")
			reply(w, 200, "GET_DATA_SUCCESS", result)
			return nil
		})
		s.register(kind+"_course_progress_controller", "export", func(w http.ResponseWriter, r *http.Request) error {
			scope, err := linkedCourseScope(r, kind)
			if err != nil {
				return err
			}
			body, err := service.Export(r.Context(), scope)
			if err != nil {
				return err
			}
			w.Header().Set("Cache-Control", "private, no-store")
			sendWorkbook(w, "Progres-kelas-"+kind+"-"+strconv.Itoa(int(scope.ID))+".xlsx", body)
			return nil
		})
	}
}
func linkedCourseScope(r *http.Request, kind string) (linkedcourse.Scope, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil || id < 1 {
		return linkedcourse.Scope{}, domain.Fail(404, "NOT_FOUND")
	}
	return linkedcourse.Scope{Kind: kind, ID: int32(id)}, nil
}
