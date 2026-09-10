package httpapi

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/storage"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

//go:embed routes.json
var routeJSON []byte

type Route struct {
	Method        string `json:"method"`
	Path          string `json:"path"`
	Controller    string `json:"controller"`
	Action        string `json:"action"`
	Auth          bool   `json:"auth"`
	TrustedOrigin bool   `json:"trusted_origin"`
	Permission    string `json:"permission"`
}
type Handler func(http.ResponseWriter, *http.Request) error
type Implementation struct {
	Route
	Implemented bool `json:"implemented"`
}

func (s *Server) ImplementationInventory() []Implementation {
	result := []Implementation{}
	for _, route := range Routes() {
		_, ok := s.handlers[route.Controller+"."+route.Action]
		result = append(result, Implementation{route, ok || route.Controller == "health"})
	}
	return result
}

type userContextKey struct{}
type Server struct {
	Config        config.Config
	Pool          *pgxpool.Pool
	Auth          *auth.Service
	Storage       storage.Store
	CourseStorage storage.Store
	Logger        *slog.Logger
	handlers      map[string]Handler
}

func Routes() []Route {
	var routes []Route
	if err := json.Unmarshal(routeJSON, &routes); err != nil {
		panic(err)
	}
	return routes
}
func (s *Server) Handler() http.Handler {
	s.handlers = map[string]Handler{}
	s.registerAuth()
	s.registerMedia()
	s.registerReference()
	s.registerAdmin()
	s.registerTickets()
	s.registerMembers()
	s.registerActivities()
	s.registerRegistrations()
	s.registerClubs()
	s.registerForms()
	s.registerClubRegistrations()
	s.registerClubRoles()
	s.registerCounseling()
	s.registerLeaderboards()
	s.registerCertificateTemplates()
	s.registerCertificates()
	s.registerCourses()
	// Adonis permits overlapping parameter paths which ServeMux rejects. Keep
	// its static-segment precedence in a small net/http compatibility dispatcher.
	type entry struct {
		route   Route
		handler http.HandlerFunc
	}
	entries := []entry{}
	routes := Routes()
	sort.SliceStable(routes, func(i, j int) bool {
		a, b := strings.Split(routes[i].Path, "/"), strings.Split(routes[j].Path, "/")
		for k := 0; k < len(a) && k < len(b); k++ {
			aw, bw := strings.HasPrefix(a[k], ":"), strings.HasPrefix(b[k], ":")
			if aw != bw {
				return !aw
			}
		}
		return len(a) > len(b)
	})
	for _, route := range routes {
		entries = append(entries, entry{route: route, handler: func(w http.ResponseWriter, r *http.Request) {
			if route.Controller == "health" {
				write(w, 200, struct {
					Status string `json:"status"`
				}{"ok"})
				return
			}
			var parseErr error
			r, parseErr = prepareRequest(w, r, route)
			if r.MultipartForm != nil {
				defer r.MultipartForm.RemoveAll()
			}
			if parseErr != nil {
				returnError(w, parseErr)
				return
			}
			if route.TrustedOrigin && r.Header.Get("Origin") != "" && !slices.Contains(s.Config.Origins, r.Header.Get("Origin")) {
				returnError(w, domain.Fail(403, "UNTRUSTED_ORIGIN"))
				return
			}
			if route.Auth {
				token := ""
				if cookie, err := r.Cookie("admin_access_jwt"); err == nil {
					token = cookie.Value
				}
				if token == "" {
					token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				}
				user, err := s.Auth.Authenticate(r.Context(), token)
				if err != nil {
					var failure *domain.Error
					if errors.As(err, &failure) && failure.Status == 401 {
						w.Header().Set("Content-Type", "text/plain; charset=utf-8")
						w.WriteHeader(401)
						_, _ = w.Write([]byte("Unauthorized access"))
						return
					}
					returnError(w, err)
					return
				}
				if route.Permission != "" && !auth.ForRole(user.RoleCode, user.IsActive).Allows(route.Permission) {
					write(w, 403, struct {
						Message    string `json:"message"`
						Permission string `json:"permission"`
					}{"FORBIDDEN", route.Permission})
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), userContextKey{}, user))
			}
			h, ok := s.handlers[route.Controller+"."+route.Action]
			if !ok {
				returnError(w, domain.Fail(501, "REWRITE_IN_PROGRESS"))
				return
			}
			if err := h(w, r); err != nil {
				var d *domain.Error
				if !errors.As(err, &d) {
					s.Logger.Error("request failed", "route", route.Path, "error", err.Error())
				}
				returnError(w, err)
			}
		}})
	}
	mux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Adonis' route matcher passes escaped path segments through to params.
		// Decoding first changes both identifier semantics and encoded slashes.
		parts := strings.Split(r.URL.EscapedPath(), "/")
		for _, entry := range entries {
			if entry.route.Method != r.Method {
				continue
			}
			pattern := strings.Split(entry.route.Path, "/")
			if len(pattern) != len(parts) {
				continue
			}
			matches := true
			for i, p := range pattern {
				if !strings.HasPrefix(p, ":") && p != parts[i] {
					matches = false
					break
				}
				if strings.HasPrefix(p, ":") && parts[i] == "" {
					matches = false
					break
				}
			}
			if !matches {
				continue
			}
			for i, p := range pattern {
				if strings.HasPrefix(p, ":") {
					r.SetPathValue(strings.TrimPrefix(p, ":"), parts[i])
				}
			}
			entry.handler(w, r)
			return
		}
		message(w, 404, "ROUTE_NOT_FOUND")
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		defer func() {
			if recovered := recover(); recovered != nil {
				s.Logger.Error("request panic", "path", r.URL.Path)
				returnError(w, domain.Fail(500, "GENERAL_ERROR"))
			}
			s.Logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(started).Milliseconds())
		}()
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Add("Vary", "Origin")
			if slices.Contains(s.Config.Origins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		if r.Method == "OPTIONS" {
			if origin != "" && slices.Contains(s.Config.Origins, origin) {
				w.Header().Set("Access-Control-Allow-Methods", "GET,HEAD,POST,PUT,PATCH,DELETE")
				w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("Access-Control-Request-Headers"))
				w.Header().Set("Access-Control-Max-Age", "90")
			}
			w.WriteHeader(204)
			return
		}
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if len(requestID) > 128 || !requestIDPattern.MatchString(requestID) {
			requestID, _ = auth.UUID()
		}
		w.Header().Set("X-Request-ID", requestID)
		r.Header.Set("X-Request-ID", requestID)
		if escaped := r.URL.EscapedPath(); escaped != "/" {
			r.URL.RawPath = strings.TrimRight(escaped, "/")
			r.URL.Path, _ = url.PathUnescape(r.URL.RawPath)
		}
		mux.ServeHTTP(w, r)
	})
}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]+$`)

func (s *Server) register(controller, action string, h Handler) {
	s.handlers[controller+"."+action] = h
}
func actor(r *http.Request) dbgen.AdminUser {
	user, _ := r.Context().Value(userContextKey{}).(dbgen.AdminUser)
	return user
}
func write(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func reply(w http.ResponseWriter, status int, message string, data interface{}) {
	write(w, status, struct {
		Message string      `json:"message"`
		Data    interface{} `json:"data"`
	}{message, data})
}
func message(w http.ResponseWriter, status int, value string) {
	write(w, status, struct {
		Message string `json:"message"`
	}{value})
}
func returnError(w http.ResponseWriter, err error) {
	var d *domain.Error
	if errors.As(err, &d) {
		if d.Fields != nil {
			body := map[string]interface{}{}
			if d.Message != "" {
				body["message"] = d.Message
			}
			for key, value := range d.Fields {
				body[key] = value
			}
			write(w, d.Status, body)
			return
		}
		message(w, d.Status, d.Message)
		return
	}
	message(w, 500, "GENERAL_ERROR")
}
