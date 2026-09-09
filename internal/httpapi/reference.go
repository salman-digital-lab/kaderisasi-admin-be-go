package httpapi

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"net/http"
)

func plural(w http.ResponseWriter, message string, data interface{}) {
	write(w, 200, struct {
		Messages string      `json:"messages"`
		Data     interface{} `json:"data"`
	}{message, data})
}
func legacyFailure(w http.ResponseWriter, err error) {
	text := err.Error()
	if errors.Is(err, pgx.ErrNoRows) {
		text = "Row not found"
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		text = pg.Message
	}
	write(w, 500, struct {
		Message string `json:"message"`
		Error   string `json:"error"`
	}{"GENERAL_ERROR", text})
}
func (s *Server) registerReference() {
	s.registerReferenceCRUD()
	s.register("countries_controller", "index", referenceHandler("GET_DATA_SUCCESS", false, func(r *http.Request) ([]dbgen.Country, error) {
		return dbgen.New(s.Pool).ListCountries(r.Context())
	}))
	s.register("rbac_roles_controller", "index", func(w http.ResponseWriter, r *http.Request) error {
		reply(w, 200, "GET_DATA_SUCCESS", auth.Roles())
		return nil
	})
	s.register("rbac_roles_controller", "show", func(w http.ResponseWriter, r *http.Request) error {
		role := auth.RoleByCode(r.PathValue("code"))
		if role == nil {
			message(w, 404, "ROLE_NOT_FOUND")
			return nil
		}
		reply(w, 200, "GET_DATA_SUCCESS", role)
		return nil
	})
	s.register("rbac_permissions_controller", "index", func(w http.ResponseWriter, r *http.Request) error {
		reply(w, 200, "GET_DATA_SUCCESS", auth.Permissions())
		return nil
	})
	s.register("rbac_permissions_controller", "requestableTargets", func(w http.ResponseWriter, r *http.Request) error {
		roles := []auth.Role{}
		for _, role := range auth.Roles() {
			if role.IsRequestable {
				roles = append(roles, role)
			}
		}
		reply(w, 200, "GET_DATA_SUCCESS", struct {
			Roles []auth.Role `json:"roles"`
		}{roles})
		return nil
	})
	s.register("dashboard_controller", "stats", referenceHandler("GET_STATS_SUCCESS", false, func(r *http.Request) (dbgen.DashboardStatsRow, error) {
		return dbgen.New(s.Pool).DashboardStats(r.Context())
	}))
	s.register("dashboard_controller", "CountProfiles", referenceHandler("GET_DATA_SUCCESS", true, func(r *http.Request) ([]dbgen.CountProfilesByLevelRow, error) {
		return dbgen.New(s.Pool).CountProfilesByLevel(r.Context())
	}))
	s.register("dashboard_controller", "CountUsersGender", referenceHandler("GET_DATA_SUCCESS", true, func(r *http.Request) ([]dbgen.CountProfilesByGenderRow, error) {
		return dbgen.New(s.Pool).CountProfilesByGender(r.Context())
	}))
}
