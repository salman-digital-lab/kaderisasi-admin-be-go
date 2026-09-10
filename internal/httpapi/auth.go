package httpapi

import (
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type validationIssue struct {
	Message string `json:"message"`
	Rule    string `json:"rule"`
	Field   string `json:"field"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type googleLoginRequest struct {
	Credential string `json:"credential"`
}

func invalid(w http.ResponseWriter, field, rule string) {
	text := "The " + field + " field must be defined"
	if rule == "email" {
		text = "The " + field + " field must be a valid email address"
	}
	if rule == "minLength" {
		text = "The " + field + " field must have at least 20 characters"
	}
	write(w, 422, struct {
		Errors []validationIssue `json:"errors"`
	}{[]validationIssue{{text, rule, field}}})
}
func clientInfo(r *http.Request) auth.ClientInfo {
	ua := r.UserAgent()
	if len(ua) > 500 {
		ua = ua[:500]
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	result := auth.ClientInfo{}
	if ua != "" {
		result.UserAgent = &ua
	}
	if ip != "" {
		result.IP = &ip
	}
	return result
}
func (s *Server) setRefresh(w http.ResponseWriter, value string) {
	http.SetCookie(w, &http.Cookie{Name: auth.RefreshCookie, Value: url.QueryEscape(auth.SignRefreshCookie(s.Config.AppKey, value)), Path: "/v2/auth", HttpOnly: true, Secure: s.Config.Environment == "production", SameSite: http.SameSiteLaxMode, MaxAge: int(auth.RefreshTTL.Seconds())})
}
func (s *Server) readRefresh(r *http.Request) string {
	cookie, err := r.Cookie(auth.RefreshCookie)
	if err != nil {
		return ""
	}
	value, _ := auth.VerifyRefreshCookie(s.Config.AppKey, cookie.Value, time.Now())
	return value
}
func (s *Server) registerAuth() {
	s.register("auth_controller", "updateProfile", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[struct {
			DisplayName *string `json:"displayName"`
		}](w, r, "editAdminUser")
		if !ok {
			return nil
		}
		if data.DisplayName == nil {
			return domain.Fail(422, "DISPLAY_NAME_REQUIRED")
		}
		user, err := dbgen.New(s.Pool).SetAdminDisplayName(r.Context(), dbgen.SetAdminDisplayNameParams{ID: actor(r).ID, DisplayName: data.DisplayName})
		if err != nil {
			return err
		}
		session, err := s.Auth.Build(r.Context(), user)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", session)
		return nil
	})
	s.register("auth_controller", "login", func(w http.ResponseWriter, r *http.Request) error {
		body, ok := inputAs[loginRequest](w, r, "loginValidator")
		if !ok {
			return nil
		}
		session, refresh, err := s.Auth.Login(r.Context(), body.Email, body.Password, clientInfo(r))
		if err != nil {
			return err
		}
		s.setRefresh(w, refresh)
		reply(w, 200, "LOGIN_SUCCESS", session)
		return nil
	})
	s.register("auth_controller", "google", func(w http.ResponseWriter, r *http.Request) error {
		if s.Config.GoogleClientID == "" {
			message(w, 503, "GOOGLE_LOGIN_NOT_CONFIGURED")
			return nil
		}
		body, ok := inputAs[googleLoginRequest](w, r, "googleLoginValidator")
		if !ok {
			return nil
		}
		session, refresh, err := s.Auth.GoogleLogin(r.Context(), body.Credential, clientInfo(r))
		if err != nil {
			return err
		}
		s.setRefresh(w, refresh)
		reply(w, 200, "LOGIN_SUCCESS", session)
		return nil
	})
	s.register("auth_controller", "refresh", func(w http.ResponseWriter, r *http.Request) error {
		session, value, err := s.Auth.Rotate(r.Context(), s.readRefresh(r), clientInfo(r))
		if err != nil {
			return err
		}
		s.setRefresh(w, value)
		reply(w, 200, "SESSION_REFRESHED", session)
		return nil
	})
	s.register("auth_controller", "me", func(w http.ResponseWriter, r *http.Request) error {
		session, err := s.Auth.Build(r.Context(), actor(r))
		if err != nil {
			return err
		}
		reply(w, 200, "GET_SESSION_SUCCESS", session)
		return nil
	})
	s.register("auth_controller", "migrate", func(w http.ResponseWriter, r *http.Request) error {
		session, value, err := s.Auth.Issue(r.Context(), actor(r), clientInfo(r))
		if err != nil {
			return err
		}
		s.setRefresh(w, value)
		reply(w, 200, "SESSION_MIGRATED", session)
		return nil
	})
	s.register("auth_controller", "logout", func(w http.ResponseWriter, r *http.Request) error {
		if err := s.Auth.Logout(r.Context(), s.readRefresh(r)); err != nil {
			return err
		}
		cookie := (&http.Cookie{Name: auth.RefreshCookie, Value: "", Path: "/v2/auth", HttpOnly: true, Secure: s.Config.Environment == "production", SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(0, 0).UTC()}).String()
		w.Header().Add("Set-Cookie", strings.Replace(cookie, "Max-Age=0", "Max-Age=-1", 1))
		message(w, 200, "LOGOUT_SUCCESS")
		return nil
	})
}
