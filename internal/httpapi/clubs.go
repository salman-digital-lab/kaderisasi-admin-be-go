package httpapi

import (
	"errors"
	"kaderisasi/admin/internal/club"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"slices"
)

func clubFailure(w http.ResponseWriter, err error) error {
	var business *domain.Error
	if errors.As(err, &business) {
		return err
	}
	legacyFailure(w, err)
	return nil
}
func (s *Server) clubService() club.Service {
	return club.Service{Pool: s.Pool, Storage: s.Storage, Location: s.Config.Location}
}
func (s *Server) registerClubs() {
	controller := "clubs_controller"
	service := s.clubService()
	s.register(controller, "delete", s.featureDeletion(service.Delete))
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		params := r.URL.Query()
		page, size := pageParams(r, 10, 0)
		filters := club.Filters{Search: params.Get("search"), Page: page, Size: size}
		if kind := params.Get("club_type"); slices.Contains([]string{"UNIT", "CLUB_KEPROFESIAN", "CLUB_BAHASA", "AVISMAN_REGIONAL"}, kind) {
			filters.ClubType = &kind
		}
		for _, field := range []struct {
			key, yes, no string
			target       **bool
		}{{"visibility", "published", "draft", &filters.IsShow}, {"registration", "open", "closed", &filters.IsRegistrationOpen}} {
			if value := params.Get(field.key); value == field.yes || value == field.no {
				flag := value == field.yes
				*field.target = &flag
			}
		}
		result, err := service.List(r.Context(), filters)
		if err != nil {
			return clubFailure(w, paginationError(r, err))
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		result, err := service.Show(r.Context(), r.PathValue("id"))
		if err != nil {
			return clubFailure(w, err)
		}
		reply(w, 200, "GET_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "store", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[club.Input](w, r, "clubValidator")
		if !ok {
			return nil
		}
		result, err := service.Create(r.Context(), data)
		if err != nil {
			return clubFailure(w, err)
		}
		reply(w, 200, "CREATE_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "update", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := inputAs[club.Input](w, r, "updateClubValidator")
		if !ok {
			return nil
		}
		result, err := service.Update(r.Context(), r.PathValue("id"), data)
		if err != nil {
			return clubFailure(w, err)
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", result)
		return nil
	})
	s.register(controller, "updateRegistrationInfo", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := caughtInputAs[club.RegistrationInfo](w, r, "updateClubRegistrationInfoValidator")
		if !ok {
			return nil
		}
		result, err := service.UpdateRegistrationInfo(r.Context(), r.PathValue("id"), data)
		if err != nil {
			return clubFailure(w, err)
		}
		reply(w, 200, "REGISTRATION_INFO_UPDATED", result)
		return nil
	})
	s.register(controller, "uploadLogo", s.uploadClubMedia)
	s.register(controller, "uploadImageMedia", s.uploadClubMedia)
	s.register(controller, "addYoutubeMedia", s.addYouTubeMedia)
	s.register(controller, "deleteMedia", s.deleteClubMedia)
}
