package httpapi

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/member"
	"net/http"
)

func (s *Server) registerMembers() {
	service := member.Service{Pool: s.Pool}
	s.register("members_controller", "store", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := validatedInputAs[member.CreateRequest](w, r, "createMemberValidator", true)
		if !ok {
			return nil
		}
		created, err := service.Create(r.Context(), data)
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
		if err := service.GenerateAccount(r.Context(), pathID(r, "id"), data.Email, data.Password); err != nil {
			var d *domain.Error
			if errors.As(err, &d) {
				return err
			}
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		reply(w, 200, "GENERATE_ACCOUNT_SUCCESS", struct {
			Email string `json:"email"`
		}{data.Email})
		return nil
	})

	s.register("auth_controller", "updateMember", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[member.CredentialRequest](w, r, "editPublicUserValidator")
		if !ok {
			return nil
		}
		updated, err := service.UpdateCredentials(r.Context(), pathID(r, "id"), data)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_MEMBER_SUCCESS", updated)
		return nil
	})
	s.registerProfileReads()
	s.register("profiles_controller", "update", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := validatedInputAs[member.ProfileUpdate](w, r, "updateProfileValidator", true)
		if !ok {
			return nil
		}
		updated, err := service.UpdateProfile(r.Context(), pathID(r, "id"), data)
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
		if err := service.UpdateRegionalAssignment(r.Context(), pathID(r, "id"), data); err != nil {
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
		removed, err := dbgen.New(s.Pool).DeleteProfile(r.Context(), pathID(r, "id"))
		if err == nil && removed == 0 {
			err = pgx.ErrNoRows
		}
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		message(w, 200, "DELETE_DATA_SUCCESS")
		return nil
	})
}
