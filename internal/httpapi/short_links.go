package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/shortlink"
)

func shortLinkInput[T any](r *http.Request) (T, error) {
	var input T
	decoder := json.NewDecoder(io.LimitReader(r.Body, 65537))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return input, domain.Fail(422, "INVALID_SHORT_LINK_INPUT")
	}
	if err := decoder.Decode(new(struct{})); err != io.EOF {
		return input, domain.Fail(422, "INVALID_SHORT_LINK_INPUT")
	}
	return input, nil
}
func (s *Server) registerShortLinks() {
	service := shortlink.Service{Pool: s.Pool, BaseURL: s.Config.ShortURLBaseURL}
	register := func(action string, h Handler) {
		s.register("short_links_controller", action, func(w http.ResponseWriter, r *http.Request) error {
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
		result, err := service.List(r.Context(), strings.TrimSpace(r.URL.Query().Get("search")), page, size)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	register("store", func(w http.ResponseWriter, r *http.Request) error {
		in, err := shortLinkInput[shortlink.Input](r)
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
	register("update", func(w http.ResponseWriter, r *http.Request) error {
		in, err := shortLinkInput[struct {
			OriginalURL string `json:"original_url"`
		}](r)
		if err != nil {
			return err
		}
		code, err := url.PathUnescape(r.PathValue("code"))
		if err != nil {
			return domain.Fail(404, "SHORT_LINK_NOT_FOUND")
		}
		result, err := service.Update(r.Context(), code, in.OriginalURL)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", result)
		return nil
	})
	register("delete", func(w http.ResponseWriter, r *http.Request) error {
		code, err := url.PathUnescape(r.PathValue("code"))
		if err != nil {
			return domain.Fail(404, "SHORT_LINK_NOT_FOUND")
		}
		if err := service.Delete(r.Context(), code); err != nil {
			return err
		}
		reply(w, 200, "DELETE_DATA_SUCCESS", nil)
		return nil
	})
}
