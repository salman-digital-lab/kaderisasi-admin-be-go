package httpapi

import (
	"encoding/json"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/talent"
	"net/http"
	"strconv"
)

func talentInput[T any](r *http.Request) (T, error) {
	var input T
	data, err := json.Marshal(requestData(r))
	if err == nil {
		err = json.Unmarshal(data, &input)
	}
	if err != nil {
		return input, domain.Fail(422, "TALENT_INVALID_REQUEST")
	}
	return input, nil
}
func (s *Server) registerTalentAssessment() {
	service := talent.Service{Pool: s.Pool}
	s.register("talent_assessment", "definition", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 200, "GET_DATA_SUCCESS", talent.PublicDefinition())
		return nil
	})
	s.register("talent_assessment", "state", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		data, err := service.State(r.Context(), actor(r).ID)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register("talent_assessment", "start", func(w http.ResponseWriter, r *http.Request) error {
		data, err := service.Start(r.Context(), actor(r).ID)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", data)
		return nil
	})
	s.register("talent_assessment", "save", func(w http.ResponseWriter, r *http.Request) error {
		input, err := talentInput[talent.SaveRequest](r)
		if err != nil {
			return err
		}
		data, err := service.Save(r.Context(), actor(r).ID, input)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", data)
		return nil
	})
	s.register("talent_assessment", "submit", func(w http.ResponseWriter, r *http.Request) error {
		input, err := talentInput[talent.SubmitRequest](r)
		if err != nil {
			return err
		}
		data, err := service.Submit(r.Context(), actor(r).ID, input)
		if err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", data)
		return nil
	})
	s.register("talent_assessment", "result", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		data, err := service.Result(r.Context(), actor(r).ID)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register("talent_assessment", "adminResult", func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Cache-Control", "no-store")
		if !auth.ForUser(actor(r)).IsSuperAdmin {
			return domain.Fail(403, "FORBIDDEN")
		}
		id, err := strconv.ParseInt(pathID(r, "id"), 10, 32)
		if err != nil || id < 1 {
			return domain.Fail(404, "DATA_NOT_FOUND")
		}
		data, err := service.Result(r.Context(), int32(id))
		if err != nil {
			return err
		}
		user, err := dbgen.New(s.Pool).FindAdminByID(r.Context(), int32(id))
		if err != nil {
			return err
		}
		name := user.Email
		if user.DisplayName != nil && *user.DisplayName != "" {
			name = *user.DisplayName
		}
		reply(w, 200, "GET_DATA_SUCCESS", struct {
			*talent.Result
			ParticipantName string `json:"participant_name"`
		}{data, name})
		return nil
	})
}
