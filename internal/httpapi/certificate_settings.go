package httpapi

import (
	"encoding/json"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"strconv"
)

func certificatePathInt(value string) (int32, error) {
	id, err := strconv.ParseInt(value, 10, 32)
	if err != nil || id <= 0 {
		return 0, domain.Fail(422, "INVALID_CERTIFICATE_ID")
	}
	return int32(id), nil
}

func (s *Server) registerCertificateSettings() {
	service := certificate.Issuance{Pool: s.Pool, Location: s.Config.Location, Logger: s.Logger}
	controller := "certificate_settings_controller"
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		id, err := certificatePathInt(r.PathValue("activityId"))
		if err != nil {
			return err
		}
		settings, err := service.Settings(r.Context(), id)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", settings)
		return nil
	})
	s.register(controller, "update", func(w http.ResponseWriter, r *http.Request) error {
		id, err := certificatePathInt(r.PathValue("activityId"))
		if err != nil {
			return err
		}
		raw, err := json.Marshal(requestData(r))
		if err != nil {
			return err
		}
		var settings certificate.Settings
		if json.Unmarshal(raw, &settings) != nil {
			return domain.Fail(422, "INVALID_CERTIFICATE_SETTINGS")
		}
		settings, err = service.SaveSettings(r.Context(), id, settings)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", settings)
		return nil
	})
	s.register(controller, "updateGroup", func(w http.ResponseWriter, r *http.Request) error {
		activityID, err := certificatePathInt(r.PathValue("activityId"))
		if err != nil {
			return err
		}
		registrationID, err := certificatePathInt(r.PathValue("registrationId"))
		if err != nil {
			return err
		}
		raw, err := json.Marshal(requestData(r))
		if err != nil {
			return err
		}
		var input struct {
			Group *string `json:"certificate_group"`
		}
		if json.Unmarshal(raw, &input) != nil {
			return domain.Fail(422, "INVALID_CERTIFICATE_GROUP")
		}
		if err := service.SaveGroup(r.Context(), activityID, registrationID, input.Group); err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", input)
		return nil
	})
}
