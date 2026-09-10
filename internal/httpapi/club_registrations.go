package httpapi

import (
	"encoding/json"
	"kaderisasi/admin/internal/club"
	"net/http"
)

func (s *Server) registerClubRegistrations() {
	controller := "club_registrations_controller"
	service := club.Service{Pool: s.Pool, Location: s.Config.Location}
	s.register(controller, "export", s.exportClubRegistrations)
	for _, action := range []string{"index", "members"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			params := r.URL.Query()
			optional := func(key string) *string {
				value := params.Get(key)
				if value == "" {
					return nil
				}
				return &value
			}
			filters := club.RegistrationFilters{ClubID: r.PathValue("id"), Status: optional("status"), Search: optional("search"), Members: action == "members", Ascending: params.Get("sort_order") == "asc", Page: queryNumber(r, "page", 1), Size: queryNumber(r, "limit", 20)}
			result, err := service.Registrations(r.Context(), filters)
			if err != nil {
				legacyFailure(w, err)
				return nil
			}
			success := "CLUB_REGISTRATIONS_RETRIEVED"
			if filters.Members {
				success = "CLUB_MEMBERS_RETRIEVED"
			}
			reply(w, 200, success, result)
			return nil
		})
	}
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.Registration(r.Context(), r.PathValue("id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "CLUB_REGISTRATION_RETRIEVED", result)
		return nil
	})
	for _, action := range []string{"store", "update"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			schema, success := "storeClubRegistrationValidator", "CLUB_REGISTRATION_CREATED"
			if action == "update" {
				schema, success = "updateClubRegistrationValidator", "CLUB_REGISTRATION_UPDATED"
			}
			input, ok := caughtInputAs[club.RegistrationInput](w, r, schema)
			if !ok {
				return nil
			}
			var result club.RegistrationResponse
			var err error
			if action == "store" {
				result, err = service.CreateRegistration(r.Context(), r.PathValue("id"), input)
			} else {
				result, err = service.UpdateRegistration(r.Context(), r.PathValue("id"), input)
			}
			if err != nil {
				return clubFailure(w, err)
			}
			reply(w, 200, success, result)
			return nil
		})
	}
	s.register(controller, "bulkUpdate", func(w http.ResponseWriter, r *http.Request) error {
		input, ok := caughtInputAs[club.RegistrationBatch](w, r, "bulkUpdateClubRegistrationsValidator")
		if !ok {
			return nil
		}
		result, err := service.BulkRegistrations(r.Context(), input)
		if err != nil {
			return clubFailure(w, err)
		}
		if len(result.Missing) > 0 {
			write(w, 404, struct {
				Message string        `json:"message"`
				IDs     []json.Number `json:"ids"`
			}{"REGISTRATIONS_NOT_FOUND", result.Missing})
			return nil
		}
		reply(w, 200, "CLUB_REGISTRATIONS_BULK_UPDATED", result.Data)
		return nil
	})
	s.register(controller, "delete", func(w http.ResponseWriter, r *http.Request) error {
		if err := service.DeleteRegistration(r.Context(), r.PathValue("id")); err != nil {
			legacyFailure(w, err)
			return nil
		}
		message(w, 200, "CLUB_REGISTRATION_DELETED")
		return nil
	})
}
func (s *Server) registerClubRoles() {
	controller := "club_member_roles_controller"
	service := club.Service{Pool: s.Pool, Location: s.Config.Location}
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.Roles(r.Context(), r.PathValue("id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "CLUB_MEMBER_ROLES_RETRIEVED", result)
		return nil
	})
	s.register(controller, "suggestions", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.RoleSuggestions(r.Context(), r.PathValue("id"))
		if err != nil {
			legacyFailure(w, err)
			return nil
		}
		reply(w, 200, "CLUB_MEMBER_ROLE_SUGGESTIONS_RETRIEVED", result)
		return nil
	})
	for _, action := range []string{"store", "update"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			schema, success, status := "updateClubMemberRoleValidator", "CLUB_MEMBER_ROLE_UPDATED", 200
			if action == "store" {
				if err := service.RequireRegistrationClub(r.Context(), r.PathValue("id")); err != nil {
					legacyFailure(w, err)
					return nil
				}
				schema, success, status = "storeClubMemberRoleValidator", "CLUB_MEMBER_ROLE_CREATED", 201
			}
			input, ok := caughtInputAs[club.RoleInput](w, r, schema)
			if !ok {
				return nil
			}
			var result club.RoleDetail
			var err error
			if action == "store" {
				result, err = service.CreateRole(r.Context(), r.PathValue("id"), input)
			} else {
				result, err = service.UpdateRole(r.Context(), r.PathValue("id"), input)
			}
			if err != nil {
				return clubFailure(w, err)
			}
			reply(w, status, success, result)
			return nil
		})
	}
	s.register(controller, "destroy", func(w http.ResponseWriter, r *http.Request) error {
		if err := service.DeleteRole(r.Context(), r.PathValue("id")); err != nil {
			legacyFailure(w, err)
			return nil
		}
		message(w, 200, "CLUB_MEMBER_ROLE_DELETED")
		return nil
	})
}
