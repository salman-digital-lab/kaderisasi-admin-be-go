package httpapi

import (
	"encoding/json"
	"kaderisasi/admin/internal/club"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/validation"
	"net/http"
	"strings"
)

func (s *Server) uploadClubMedia(w http.ResponseWriter, r *http.Request) error {
	logo := strings.HasSuffix(r.URL.Path, "/logo")
	limit := int64(5 << 20)
	if logo {
		limit = 2 << 20
	}
	body, err := readImage(w, r, limit)
	if err != nil {
		return err
	}
	if !logo {
		raw, _ := json.Marshal(r.FormValue("media_type"))
		if _, issues := validation.ValidateField("imageMediaValidator", "media_type", raw); len(issues) != 0 {
			return domain.Details(422, "", map[string]interface{}{"errors": issues})
		}
	}
	result, err := s.clubService().Upload(r.Context(), r.PathValue("id"), body, logo)
	if err != nil {
		return err
	}
	success := "UPLOAD_MEDIA_SUCCESS"
	if logo {
		success = "UPLOAD_LOGO_SUCCESS"
	}
	reply(w, 200, success, result)
	return nil
}
func (s *Server) addYouTubeMedia(w http.ResponseWriter, r *http.Request) error {
	data, ok := inputAs[club.MediaRequest](w, r, "youtubeMediaValidator")
	if !ok {
		return nil
	}
	result, err := s.clubService().AddYouTube(r.Context(), r.PathValue("id"), data)
	if err != nil {
		return clubFailure(w, err)
	}
	reply(w, 200, "ADD_YOUTUBE_MEDIA_SUCCESS", result)
	return nil
}
func (s *Server) deleteClubMedia(w http.ResponseWriter, r *http.Request) error {
	data, ok := caughtInputAs[club.MediaRequest](w, r, "deleteClubMediaValidator")
	if !ok {
		return nil
	}
	result, err := s.clubService().DeleteMedia(r.Context(), r.PathValue("id"), data)
	if err != nil {
		return clubFailure(w, err)
	}
	reply(w, 200, "DELETE_MEDIA_SUCCESS", result)
	return nil
}
