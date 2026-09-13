package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/scoring"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func scoringID(r *http.Request, key string) (int32, error) {
	id, err := strconv.ParseInt(r.PathValue(key), 10, 32)
	if err != nil || id < 1 {
		return 0, domain.Fail(404, "SCORING_RESOURCE_NOT_FOUND")
	}
	return int32(id), nil
}
func scoringInput[T any](r *http.Request) (T, error) {
	var in T
	raw, err := json.Marshal(requestData(r))
	if err != nil {
		return in, err
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&in); err != nil {
		return in, domain.Fail(422, "INVALID_SCORING_INPUT")
	}
	return in, nil
}
func (s *Server) registerScoring() {
	service := scoring.Service{Pool: s.Pool}
	register := func(action string, h func(http.ResponseWriter, *http.Request, int32) error) {
		s.register("scoring_controller", action, func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Cache-Control", "private, no-store")
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if cookie, err := r.Cookie("admin_access_jwt"); err == nil && cookie.Value != "" {
				token = cookie.Value
			}
			claims, err := auth.VerifyAccess(s.Auth.Key, token, time.Now())
			if err != nil || len(claims.Audience) != 1 || claims.Audience[0] != auth.AdminAudience {
				return domain.Fail(401, "UNAUTHORIZED")
			}
			id, err := scoringID(r, "id")
			if err != nil {
				return err
			}
			return h(w, r, id)
		})
	}
	register("rubric", func(w http.ResponseWriter, r *http.Request, id int32) error {
		result, err := service.Rubric(r.Context(), id)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	register("saveRubric", func(w http.ResponseWriter, r *http.Request, id int32) error {
		in, err := scoringInput[scoring.Rubric](r)
		if err != nil {
			return err
		}
		result, err := service.SaveRubric(r.Context(), id, actor(r).ID, in)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", result)
		return nil
	})
	register("index", func(w http.ResponseWriter, r *http.Request, id int32) error {
		page, size := coursePagination(r)
		result, err := service.List(r.Context(), id, r.URL.Query().Get("search"), r.URL.Query().Get("state"), page, size)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	register("save", func(w http.ResponseWriter, r *http.Request, id int32) error {
		registrationID, err := scoringID(r, "registrationId")
		if err != nil {
			return err
		}
		in, err := scoringInput[scoring.SaveInput](r)
		if err != nil {
			return err
		}
		result, err := service.Save(r.Context(), id, registrationID, actor(r).ID, in)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", result)
		return nil
	})
	for _, action := range []string{"publish", "withdraw"} {
		register(action, func(w http.ResponseWriter, r *http.Request, id int32) error {
			in, err := scoringInput[scoring.Batch](r)
			if err != nil {
				return err
			}
			if err = service.Publish(r.Context(), id, actor(r).ID, in, action == "withdraw"); err != nil {
				return err
			}
			reply(w, 200, "UPDATE_DATA_SUCCESS", nil)
			return nil
		})
	}
	register("excel", func(w http.ResponseWriter, r *http.Request, id int32) error {
		body, err := service.Workbook(r.Context(), id, r.URL.Query().Get("mode"))
		if err != nil {
			return err
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="activity-scoring.xlsx"`)
		w.WriteHeader(200)
		_, err = w.Write(body)
		return err
	})
	for _, action := range []string{"preview", "commit"} {
		register(action, func(w http.ResponseWriter, r *http.Request, id int32) error {
			file, _, err := r.FormFile("file")
			if err != nil {
				return domain.Fail(422, "SCORING_WORKBOOK_REQUIRED")
			}
			defer file.Close()
			body, err := io.ReadAll(io.LimitReader(file, scoring.MaxWorkbookBytes+1))
			if err != nil {
				return err
			}
			result, err := service.Import(r.Context(), id, actor(r).ID, body, r.FormValue("preview_hash"), action == "commit")
			if err != nil {
				return err
			}
			reply(w, 200, "GET_DATA_SUCCESS", result)
			return nil
		})
	}
}
