package httpapi

import (
	"encoding/json"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/form"
	"net/http"
)

func (s *Server) registerForms() {
	s.registerFormReads()
	controller := "custom_forms_controller"
	service := form.Service{Pool: s.Pool, Location: s.Config.Location}
	s.register(controller, "store", func(w http.ResponseWriter, r *http.Request) error {
		input, ok := caughtInputAs[form.Input](w, r, "customFormValidator")
		if !ok {
			return nil
		}
		result, err := service.Create(r.Context(), input)
		if err != nil {
			return clubFailure(w, err)
		}
		reply(w, 201, "CUSTOM_FORM_CREATED_SUCCESS", result)
		return nil
	})
	for _, action := range []string{"update", "attachToClub", "detachFromClub"} {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			var input form.Input
			success := "CUSTOM_FORM_UPDATED_SUCCESS"
			switch action {
			case "update":
				var ok bool
				input, ok = caughtInputAs[form.Input](w, r, "updateCustomFormValidator")
				if !ok {
					return nil
				}
			case "attachToClub":
				attachment, ok := caughtInputAs[form.ClubAttachment](w, r, "attachCustomFormToClubValidator")
				if !ok {
					return nil
				}
				kind := "club_registration"
				input.FeatureType = &kind
				input.FeatureID = domain.Value(attachment.ClubID)
				success = "FORM_ATTACHED_TO_CLUB_SUCCESS"
			case "detachFromClub":
				input.FeatureID = domain.Optional[json.Number]{Present: true}
				success = "FORM_DETACHED_FROM_CLUB_SUCCESS"
			}
			result, err := service.Update(r.Context(), r.PathValue("id"), input, action == "attachToClub")
			if err != nil {
				return clubFailure(w, err)
			}
			reply(w, 200, success, result)
			return nil
		})
	}
	s.register(controller, "destroy", func(w http.ResponseWriter, r *http.Request) error {
		if err := service.Delete(r.Context(), r.PathValue("id")); err != nil {
			return clubFailure(w, err)
		}
		message(w, 200, "CUSTOM_FORM_DELETED_SUCCESS")
		return nil
	})
	s.register(controller, "toggleActive", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.Toggle(r.Context(), r.PathValue("id"))
		if err != nil {
			return clubFailure(w, err)
		}
		reply(w, 200, "CUSTOM_FORM_STATUS_UPDATED", result)
		return nil
	})
	s.register(controller, "attachToActivity", func(w http.ResponseWriter, r *http.Request) error {
		input, ok := decodeInputAs[form.ActivityAttachment](w, requestData(r))
		if !ok {
			return nil
		}
		result, err := service.AttachActivity(r.Context(), r.PathValue("id"), input)
		if err != nil {
			return clubFailure(w, err)
		}
		reply(w, 200, "FORM_ATTACHED_TO_ACTIVITY_SUCCESS", result)
		return nil
	})
	s.register(controller, "detachFromActivity", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.DetachActivity(r.Context(), r.PathValue("id"))
		if err != nil {
			return clubFailure(w, err)
		}
		reply(w, 200, "FORM_DETACHED_FROM_ACTIVITY_SUCCESS", result)
		return nil
	})
}
