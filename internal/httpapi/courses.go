package httpapi

import (
	"encoding/json"
	"io"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/course"
	"kaderisasi/admin/internal/domain"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func courseID(r *http.Request, key string) (int32, error) {
	value, err := strconv.ParseInt(r.PathValue(key), 10, 32)
	if err != nil || value < 1 {
		return 0, domain.Fail(404, "COURSE_NOT_FOUND")
	}
	return int32(value), nil
}
func courseInput[T any](r *http.Request) (T, error) {
	var result T
	raw, err := json.Marshal(requestData(r))
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return result, domain.Fail(422, "INVALID_COURSE_INPUT")
	}
	return result, nil
}
func coursePagination(r *http.Request) (int32, int32) {
	number, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || number < 1 {
		number = 1
	}
	number = min(number, 1000000)
	size, err := strconv.Atoi(r.URL.Query().Get("per_page"))
	if err != nil || size < 1 {
		size = 12
	}
	size = min(size, 100)
	return int32(number), int32(size)
}
func (s *Server) registerCourses() {
	service := course.Service{Pool: s.Pool, Storage: s.CourseStorage}
	register := func(action string, h Handler) {
		s.register("courses_controller", action, func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Cache-Control", "private, no-store")
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if cookie, err := r.Cookie("admin_access_jwt"); err == nil && cookie.Value != "" {
				token = cookie.Value
			}
			claims, err := auth.VerifyAccess(s.Auth.Key, token, time.Now())
			if err != nil || len(claims.Audience) != 1 || claims.Audience[0] != auth.AdminAudience {
				return domain.Fail(401, "UNAUTHORIZED")
			}
			return h(w, r)
		})
	}
	register("index", func(w http.ResponseWriter, r *http.Request) error {
		page, size := coursePagination(r)
		result, err := service.List(r.Context(), strings.TrimSpace(r.URL.Query().Get("search")), r.URL.Query().Get("status"), page, size)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	register("store", func(w http.ResponseWriter, r *http.Request) error {
		in, err := courseInput[course.Input](r)
		if err != nil {
			return err
		}
		result, err := service.Create(r.Context(), in)
		if err != nil {
			return err
		}
		reply(w, 201, "CREATE_DATA_SUCCESS", result)
		return nil
	})
	register("show", func(w http.ResponseWriter, r *http.Request) error {
		id, err := courseID(r, "id")
		if err != nil {
			return err
		}
		result, err := service.Show(r.Context(), id)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	register("update", func(w http.ResponseWriter, r *http.Request) error {
		id, err := courseID(r, "id")
		if err != nil {
			return err
		}
		in, err := courseInput[course.Input](r)
		if err != nil {
			return err
		}
		result, err := service.Update(r.Context(), id, in)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", result)
		return nil
	})
	saveLesson := func(w http.ResponseWriter, r *http.Request) error {
		id, err := courseID(r, "id")
		if err != nil {
			return err
		}
		var lessonID int32
		if r.Method == "PUT" {
			lessonID, err = courseID(r, "lessonId")
			if err != nil {
				return err
			}
		}
		in, err := courseInput[course.LessonInput](r)
		if err != nil {
			return err
		}
		result, err := service.SaveLesson(r.Context(), id, lessonID, in)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", result)
		return nil
	}
	register("createLesson", saveLesson)
	register("updateLesson", saveLesson)
	register("removeLesson", func(w http.ResponseWriter, r *http.Request) error {
		id, err := courseID(r, "id")
		if err != nil {
			return err
		}
		lessonID, err := courseID(r, "lessonId")
		if err != nil {
			return err
		}
		if err = service.RemoveLesson(r.Context(), id, lessonID); err != nil {
			return err
		}
		reply(w, 200, "DELETE_DATA_SUCCESS", nil)
		return nil
	})
	register("reorder", func(w http.ResponseWriter, r *http.Request) error {
		id, err := courseID(r, "id")
		if err != nil {
			return err
		}
		in, err := courseInput[course.OrderInput](r)
		if err != nil {
			return err
		}
		if err = service.Reorder(r.Context(), id, in.LessonIDs); err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", nil)
		return nil
	})
	register("learners", func(w http.ResponseWriter, r *http.Request) error {
		id, err := courseID(r, "id")
		if err != nil {
			return err
		}
		page, size := coursePagination(r)
		result, err := service.Learners(r.Context(), id, strings.TrimSpace(r.URL.Query().Get("search")), page, size)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	register("uploadDocument", func(w http.ResponseWriter, r *http.Request) error {
		id, err := courseID(r, "id")
		if err != nil {
			return err
		}
		lessonID, err := courseID(r, "lessonId")
		if err != nil {
			return err
		}
		if r.MultipartForm == nil {
			return domain.Fail(422, "PDF_REQUIRED")
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			return domain.Fail(422, "PDF_REQUIRED")
		}
		defer file.Close()
		if header.Size > course.MaxPDFBytes {
			return domain.Fail(422, "INVALID_PDF")
		}
		body, err := io.ReadAll(io.LimitReader(file, course.MaxPDFBytes+1))
		if err != nil {
			return err
		}
		result, err := service.Upload(r.Context(), id, lessonID, header.Filename, body)
		if err != nil {
			return err
		}
		reply(w, 201, "UPLOAD_FILE_SUCCESS", result)
		return nil
	})
	register("downloadDocument", func(w http.ResponseWriter, r *http.Request) error {
		id, err := courseID(r, "id")
		if err != nil {
			return err
		}
		lessonID, err := courseID(r, "lessonId")
		if err != nil {
			return err
		}
		documentID, err := courseID(r, "documentId")
		if err != nil {
			return err
		}
		document, body, err := service.Download(r.Context(), id, lessonID, documentID)
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": document.Filename}))
		w.WriteHeader(200)
		_, err = w.Write(body)
		return err
	})
	register("removeDocument", func(w http.ResponseWriter, r *http.Request) error {
		id, err := courseID(r, "id")
		if err != nil {
			return err
		}
		lessonID, err := courseID(r, "lessonId")
		if err != nil {
			return err
		}
		documentID, err := courseID(r, "documentId")
		if err != nil {
			return err
		}
		if err = service.RemoveDocument(r.Context(), id, lessonID, documentID); err != nil {
			return err
		}
		reply(w, 200, "DELETE_DATA_SUCCESS", nil)
		return nil
	})
}
