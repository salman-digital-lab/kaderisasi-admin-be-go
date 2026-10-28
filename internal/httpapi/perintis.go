package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/perintis"
)

// Pembinaan Beasiswa Perintis for the pengelola. Permissions come from the
// route table; responses use the camelCase shapes of kaderisasi-perintis-fe
// and failures carry Indonesian messages the app shows as is.
func (s *Server) registerPerintis() {
	service := &perintis.StaffService{Pool: s.Pool, Storage: s.Storage, Now: s.perintisClock()}
	register := func(action string, h func(http.ResponseWriter, *http.Request) (any, error)) {
		s.register("perintis_controller", action, func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Cache-Control", "private, no-store")
			data, err := h(w, r)
			if err != nil {
				var failure *perintis.Error
				if errors.As(err, &failure) {
					return domain.Fail(failure.Status, failure.Message)
				}
				return err
			}
			if data != nil {
				reply(w, 200, "OK", data)
			}
			return nil
		})
	}
	id := func(r *http.Request, name string) (int32, error) {
		n, err := strconv.ParseInt(r.PathValue(name), 10, 32)
		if err != nil || n <= 0 {
			return 0, &perintis.Error{Status: 404, Message: "Data tidak ditemukan."}
		}
		return int32(n), nil
	}
	query := func(r *http.Request, key string) string { return r.URL.Query().Get(key) }
	ok := struct {
		OK bool `json:"ok"`
	}{true}

	register("dashboard", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		return service.Dashboard(r.Context(), query(r, "month"))
	})
	register("followups", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		return service.Followups(r.Context(), query(r, "source"), query(r, "status"))
	})
	register("resolveFollowup", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r, "id")
		if err != nil {
			return nil, err
		}
		var in struct {
			Note string `json:"note"`
		}
		if err := perintisInput(r, &in); err != nil {
			return nil, err
		}
		return ok, service.ResolveFollowup(r.Context(), actor(r).ID, r.PathValue("source"), target, in.Note)
	})
	register("participants", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		return service.Participants(r.Context(), query(r, "role"), query(r, "status"), query(r, "q"))
	})
	register("participant", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r, "id")
		if err != nil {
			return nil, err
		}
		return service.Participant(r.Context(), target)
	})
	register("searchMembers", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		return service.SearchMembers(r.Context(), query(r, "q"))
	})
	register("addParticipant", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in perintis.AddParticipantInput
		if err := perintisInput(r, &in); err != nil {
			return nil, err
		}
		return service.AddParticipant(r.Context(), actor(r).ID, in)
	})
	register("updateParticipant", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r, "id")
		if err != nil {
			return nil, err
		}
		var in perintis.UpdateParticipantInput
		if err := perintisInput(r, &in); err != nil {
			return nil, err
		}
		return service.UpdateParticipant(r.Context(), actor(r).ID, target, in)
	})
	register("moveMentee", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r, "id")
		if err != nil {
			return nil, err
		}
		var in struct {
			MentorUserID *int32 `json:"mentorUserId"`
		}
		if err := perintisInput(r, &in); err != nil {
			return nil, err
		}
		return service.MoveMentee(r.Context(), actor(r).ID, target, in.MentorUserID)
	})
	register("setStatus", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r, "id")
		if err != nil {
			return nil, err
		}
		var in struct {
			Status string `json:"status"`
			Reason string `json:"reason"`
		}
		if err := perintisInput(r, &in); err != nil {
			return nil, err
		}
		return service.SetStatus(r.Context(), actor(r).ID, target, in.Status, in.Reason)
	})
	register("journal", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r, "id")
		if err != nil {
			return nil, err
		}
		return service.Journal(r.Context(), target)
	})
	register("reopenJournal", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			MenteeUserID int32  `json:"menteeUserId"`
			Month        string `json:"month"`
			Reason       string `json:"reason"`
		}
		if err := perintisInput(r, &in); err != nil {
			return nil, err
		}
		return ok, service.Reopen(r.Context(), actor(r).ID, in.MenteeUserID, in.Month, in.Reason)
	})
	register("importPreview", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Kind string `json:"kind"`
			CSV  string `json:"csv"`
		}
		if err := perintisInput(r, &in); err != nil {
			return nil, err
		}
		return service.ImportPreview(r.Context(), in.Kind, in.CSV)
	})
	register("importApply", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		var in struct {
			Kind string `json:"kind"`
			CSV  string `json:"csv"`
		}
		if err := perintisInput(r, &in); err != nil {
			return nil, err
		}
		applied, err := service.ImportApply(r.Context(), actor(r).ID, in.Kind, in.CSV)
		if err != nil {
			return nil, err
		}
		return struct {
			Applied int `json:"applied"`
		}{applied}, nil
	})
	register("export", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		filename, csv, err := service.Export(r.Context(), r.PathValue("kind"), query(r, "month"))
		if err != nil {
			return nil, err
		}
		return struct {
			Filename string `json:"filename"`
			CSV      string `json:"csv"`
		}{filename, csv}, nil
	})
	register("file", func(w http.ResponseWriter, r *http.Request) (any, error) {
		data, mime, err := service.File(r.Context(), query(r, "key"))
		if err != nil {
			return nil, err
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", "inline")
		_, _ = w.Write(data)
		return nil, nil
	})
}

// perintisClock honours PERINTIS_CLOCK by shifting real time so the demo
// clock still moves.
func (s *Server) perintisClock() func() time.Time {
	if s.Config.PerintisClock == "" {
		return time.Now
	}
	start, err := time.Parse(time.RFC3339, s.Config.PerintisClock)
	if err != nil {
		s.Logger.Warn("ignoring invalid PERINTIS_CLOCK")
		return time.Now
	}
	offset := time.Until(start)
	return func() time.Time { return time.Now().Add(offset) }
}

func perintisInput(r *http.Request, into any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return &perintis.Error{Status: 413, Message: "Data terlalu besar."}
	}
	if err := json.Unmarshal(body, into); err != nil {
		return &perintis.Error{Status: 422, Message: "Data tidak terbaca."}
	}
	return nil
}
