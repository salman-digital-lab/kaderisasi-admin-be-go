package httpapi

import (
	"errors"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/member"
	"kaderisasi/admin/internal/validation"
	"net/http"
	"strconv"
	"strings"
)

// memberActor attributes a member change to the signed-in admin for the audit trail.
func memberActor(r *http.Request, source string) member.Actor {
	admin := actor(r)
	name := admin.Email
	if admin.DisplayName != nil && strings.TrimSpace(*admin.DisplayName) != "" {
		name = strings.TrimSpace(*admin.DisplayName)
	}
	return member.Actor{AdminID: admin.ID, Name: name, RequestID: r.Header.Get("X-Request-ID"), Source: source}
}

func (s *Server) registerMembers() {
	service := member.Service{Pool: s.Pool}
	// Read-only trail of the writes below (docs/MEMBER_AUDIT_LOG.md).
	s.register("member_audit_controller", "index", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "private, no-store")
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
		if err != nil || id <= 0 {
			return domain.Fail(404, "Anggota tidak ditemukan.")
		}
		page, perPage := pageParams(r, 20, 100)
		if !(page >= 1) || !(perPage >= 1) || page > 1e6 {
			return domain.Fail(422, "Halaman tidak valid.")
		}
		logs, err := service.AuditLogs(r.Context(), int32(id), int32(page), int32(perPage))
		if err != nil {
			return err
		}
		reply(w, 200, "OK", logs)
		return nil
	})
	s.register("members_controller", "store", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := validatedInputAs[member.CreateRequest](w, r, "createMemberValidator", true)
		if !ok {
			return nil
		}
		created, err := service.Create(r.Context(), memberActor(r, "admin"), data)
		if err != nil {
			var d *domain.Error
			if errors.As(err, &d) {
				return err
			}
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		reply(w, 201, "CREATE_MEMBER_SUCCESS", created)
		return nil
	})
	s.register("members_controller", "generateAccount", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := validatedInputAs[loginRequest](w, r, "generateAccountValidator", true)
		if !ok {
			return nil
		}
		if err := service.GenerateAccount(r.Context(), memberActor(r, "admin"), pathID(r, "id"), data.Email, data.Password); err != nil {
			var d *domain.Error
			if errors.As(err, &d) {
				return err
			}
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		reply(w, 200, "GENERATE_ACCOUNT_SUCCESS", struct {
			Email string `json:"email"`
		}{*member.NormalizeEmail(&data.Email)})
		return nil
	})

	s.register("auth_controller", "updateMember", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[member.CredentialRequest](w, r, "editPublicUserValidator")
		if !ok {
			return nil
		}
		updated, err := service.UpdateCredentials(r.Context(), memberActor(r, "admin"), pathID(r, "id"), data)
		if err != nil {
			return s.frameworkError(err, "")
		}
		reply(w, 200, "UPDATE_MEMBER_SUCCESS", updated)
		return nil
	})
	s.registerProfileReads()
	s.registerMemberExport()
	s.register("profiles_controller", "update", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := validatedInputAs[member.ProfileUpdate](w, r, "memberProfileUpdateValidator", true)
		if !ok {
			return nil
		}
		updated, err := service.UpdateProfile(r.Context(), memberActor(r, "admin"), pathID(r, "id"), data)
		var unlisted *member.InstitutionError
		if errors.As(err, &unlisted) {
			// Same envelope as this route's other validation failures.
			issues := []validation.Issue{{Message: unlisted.Error(), Rule: "database.exists", Field: unlisted.Field()}}
			write(w, 500, struct {
				Message string             `json:"message"`
				Error   []validation.Issue `json:"error"`
			}{issues[0].Message, issues})
			return nil
		}
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", updated)
		return nil
	})
	s.register("profiles_controller", "updateRegionalAssignment", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := validatedInputAs[member.RegionalRequest](w, r, "regionalAssignmentValidator", true)
		if !ok {
			return nil
		}
		if err := service.UpdateRegionalAssignment(r.Context(), memberActor(r, "admin"), pathID(r, "id"), data); err != nil {
			var d *domain.Error
			if errors.As(err, &d) {
				return err
			}
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", data)
		return nil
	})
	s.register("profiles_controller", "delete", func(w http.ResponseWriter, r *http.Request) error {
		err := service.Delete(r.Context(), memberActor(r, "admin"), pathID(r, "id"))
		err = database.LegacyQueryError(err, `select * from "profiles" where "id" = $1 limit $2`)
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		message(w, 200, "DELETE_DATA_SUCCESS")
		return nil
	})
}
