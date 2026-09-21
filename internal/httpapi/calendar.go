package httpapi

import (
	"encoding/json"
	"io"
	"kaderisasi/admin/internal/calendar"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"strconv"
)

func calendarID(r *http.Request) (int32, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil || id <= 0 {
		return 0, domain.Fail(404, "CALENDAR_EVENT_NOT_FOUND")
	}
	return int32(id), nil
}
func (s *Server) registerCalendarEvents() {
	service := calendar.Service{Queries: dbgen.New(s.Pool)}
	register := func(action string, h Handler) {
		s.register("calendar_events_controller", action, func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Cache-Control", "no-store")
			return h(w, r)
		})
	}
	for _, action := range []string{"public", "index"} {
		register(action, func(w http.ResponseWriter, r *http.Request) error {
			start, end, err := calendar.Range(r.URL.Query().Get("start"), r.URL.Query().Get("end"))
			if err != nil {
				return err
			}
			events, err := service.List(r.Context(), start, end, action == "public")
			if err != nil {
				return err
			}
			reply(w, 200, "GET_DATA_SUCCESS", events)
			return nil
		})
	}
	register("show", func(w http.ResponseWriter, r *http.Request) error {
		id, err := calendarID(r)
		if err != nil {
			return err
		}
		event, err := service.Get(r.Context(), id)
		if err != nil {
			return err
		}
		reply(w, 200, "GET_DATA_SUCCESS", event)
		return nil
	})
	for _, action := range []string{"store", "update"} {
		register(action, func(w http.ResponseWriter, r *http.Request) error {
			var id int32
			if action == "update" {
				var err error
				id, err = calendarID(r)
				if err != nil {
					return err
				}
			}
			var in calendar.Input
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&in); err != nil {
				return domain.Fail(422, "INVALID_CALENDAR_EVENT")
			}
			if err := decoder.Decode(new(struct{})); err != io.EOF {
				return domain.Fail(422, "INVALID_CALENDAR_EVENT")
			}
			event, err := service.Save(r.Context(), id, in)
			if err != nil {
				return err
			}
			if action == "store" {
				reply(w, 201, "CREATE_DATA_SUCCESS", event)
			} else {
				reply(w, 200, "UPDATE_DATA_SUCCESS", event)
			}
			return nil
		})
	}
	register("delete", func(w http.ResponseWriter, r *http.Request) error {
		id, err := calendarID(r)
		if err != nil {
			return err
		}
		if err = service.Delete(r.Context(), id); err != nil {
			return err
		}
		reply(w, 200, "DELETE_DATA_SUCCESS", nil)
		return nil
	})
}
