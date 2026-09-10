package httpapi

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/access"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"strings"
	"time"
)

func (s *Server) adminView(ctx context.Context, user dbgen.AdminUser) (adminResponse, error) {
	identities, err := dbgen.New(s.Pool).AdminIdentities(ctx, user.ID)
	if err != nil {
		return adminResponse{}, err
	}
	methods := []string{}
	if user.Password != nil && *user.Password != "" {
		methods = append(methods, "password")
	}
	views := make([]adminIdentityResponse, len(identities))
	google := false
	for i, identity := range identities {
		methods = append(methods, identity.Provider)
		if identity.Provider == "google" {
			google = true
		}
		views[i] = adminIdentityResponse{Provider: identity.Provider, Email: identity.Email, LastUsedAt: timestamp(identity.LastUsedAt, time.UTC), CreatedAt: timestamp(identity.CreatedAt, time.UTC)}
	}
	a := auth.ForRole(user.RoleCode, user.IsActive)
	return adminResponse{ID: user.ID, Email: user.Email, NormalizedEmail: user.NormalizedEmail, DisplayName: user.DisplayName, CreatedAt: domain.ModelTimestamp(user.CreatedAt, s.Config.Location), UpdatedAt: domain.ModelTimestamp(user.UpdatedAt, s.Config.Location), IsActive: user.IsActive, RoleCode: user.RoleCode, Role: a.Role, EffectivePermissions: a.Permissions, IsSuperAdmin: a.IsSuperAdmin, AuthenticationMethods: methods, GoogleLinked: google, Identities: views}, nil
}
func (s *Server) adminReply(w http.ResponseWriter, r *http.Request, status int, msg string, id int32) error {
	user, err := dbgen.New(s.Pool).FindAdminByID(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "USER_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	view, err := s.adminView(r.Context(), user)
	if err != nil {
		return err
	}
	reply(w, status, msg, view)
	return nil
}
func (s *Server) registerAdmin() {
	s.register("adminusers_controller", "index", func(w http.ResponseWriter, r *http.Request) error {
		page, size := pageParams(r, 10, 100)
		search := "%" + strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search"))) + "%"
		q := dbgen.New(s.Pool)
		total, err := q.CountAdmins(r.Context(), search)
		if err != nil {
			return err
		}
		users := []dbgen.AdminUser{}
		if total > 0 {
			limit, offset, pageErr := database.SQLPage(page, size)
			if pageErr != nil {
				return pageErr
			}
			users, err = q.ListAdmins(r.Context(), dbgen.ListAdminsParams{Search: search, PageSize: limit, PageOffset: offset})
			if err != nil {
				return domain.Fail(500, paginationError(r, err).Error())
			}
		}
		data := adminPage{Meta: database.Meta(total, page, size), Data: make([]adminResponse, len(users))}
		for i, user := range users {
			data.Data[i], err = s.adminView(r.Context(), user)
			if err != nil {
				return err
			}
		}
		reply(w, 200, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register("adminusers_controller", "show", func(w http.ResponseWriter, r *http.Request) error {
		return s.adminReply(w, r, 200, "GET_DATA_SUCCESS", pathID(r, "id"))
	})
	s.register("adminusers_controller", "create", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[adminCreateRequest](w, r, "registerValidator")
		if !ok {
			return nil
		}
		email := strings.ToLower(strings.TrimSpace(data.Email))
		_, err := dbgen.New(s.Pool).FindAdminByEmail(r.Context(), email)
		if err == nil {
			return domain.Fail(409, "EMAIL_ALREADY_REGISTERED")
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		hash, err := auth.HashPassword(data.Password)
		if err != nil {
			return err
		}
		user, err := dbgen.New(s.Pool).CreateAdmin(r.Context(), dbgen.CreateAdminParams{Email: email, Password: &hash, DisplayName: &data.DisplayName, RoleCode: data.RoleCode})
		if err != nil {
			return err
		}
		return s.adminReply(w, r, 201, "REGISTER_SUCCESS", user.ID)
	})
	s.register("adminusers_controller", "editPassword", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[passwordRequest](w, r, "editPasswordValidator")
		if !ok {
			return nil
		}
		id := pathID(r, "id")
		if _, err := dbgen.New(s.Pool).FindAdminByID(r.Context(), id); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.Fail(404, "USER_NOT_FOUND")
			}
			return err
		}
		hash, err := auth.HashPassword(data.Password)
		if err != nil {
			return err
		}
		if err = dbgen.New(s.Pool).SetAdminPassword(r.Context(), dbgen.SetAdminPasswordParams{ID: id, Password: &hash}); err != nil {
			return err
		}
		reason := "password_reset"
		if err = dbgen.New(s.Pool).RevokeAdminSessions(r.Context(), dbgen.RevokeAdminSessionsParams{AdminUserID: id, RevocationReason: &reason}); err != nil {
			return err
		}
		message(w, 200, "RESET_PASSWORD_SUCCESS")
		return nil
	})
	s.register("adminusers_controller", "update", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[adminUpdateRequest](w, r, "editAdminUser")
		if !ok {
			return nil
		}
		id := pathID(r, "id")
		currentActor := actor(r)
		if data.IsActive.Present && data.IsActive.Value != nil && !*data.IsActive.Value && id == currentActor.ID {
			return domain.Fail(409, "SELF_DEACTIVATION_NOT_ALLOWED")
		}
		change := access.Update{RoleCode: data.RoleCode, IsActive: data.IsActive}
		tx, err := s.Pool.Begin(r.Context())
		if err != nil {
			return err
		}
		defer tx.Rollback(r.Context())
		if err = dbgen.New(tx).AcquireRBACLock(r.Context()); err != nil {
			return err
		}
		fresh, err := dbgen.New(tx).FindAdminByID(r.Context(), currentActor.ID)
		if err != nil {
			return err
		}
		if !fresh.IsActive || fresh.RoleCode == nil || *fresh.RoleCode != "super_admin" {
			return domain.Fail(403, "SUPER_ADMIN_REQUIRED")
		}
		if err = access.Change(r.Context(), tx, id, change); err != nil {
			return err
		}
		if err = tx.Commit(r.Context()); err != nil {
			return err
		}
		return s.adminReply(w, r, 200, "UPDATE_DATA_SUCCESS", id)
	})
}
