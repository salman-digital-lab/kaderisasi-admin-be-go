package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/member"
	"kaderisasi/admin/internal/perintis"
	"kaderisasi/admin/internal/validation"
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
		return service.Participants(r.Context(), query(r, "role"), query(r, "status"), query(r, "mentor"), query(r, "q"))
	})
	register("participant", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r, "id")
		if err != nil {
			return nil, err
		}
		return service.Participant(r.Context(), target)
	})
	// A participant's kaderisasi profile, in the same shape and with the same
	// validation as the admin member page (/v2/profiles), limited to Perintis
	// participants. Account fields (password) and extra_data are not editable
	// here.
	members := member.Service{Pool: s.Pool}
	profileOf := func(r *http.Request, user int32) (*member.ProfileDetail, error) {
		rows, err := dbgen.New(s.Pool).ProfileDetailsByIdentifier(r.Context(), dbgen.ProfileDetailsByIdentifierParams{ID: strconv.Itoa(int(user)), ByUser: true})
		if err != nil || len(rows) == 0 {
			return nil, err
		}
		detail, err := member.ProfileWithRelations(rows[0])
		return &detail, err
	}
	type participantProfile struct {
		Profile *member.ProfileDetail `json:"profile"`
	}
	register("participantProfile", func(_ http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r, "id")
		if err != nil {
			return nil, err
		}
		if err := service.RequireParticipant(r.Context(), target); err != nil {
			return nil, err
		}
		profile, err := profileOf(r, target)
		return participantProfile{Profile: profile}, err
	})
	register("updateParticipantProfile", func(w http.ResponseWriter, r *http.Request) (any, error) {
		target, err := id(r, "id")
		if err != nil {
			return nil, err
		}
		if err := service.RequireParticipant(r.Context(), target); err != nil {
			return nil, err
		}
		current, err := profileOf(r, target)
		if err != nil {
			return nil, err
		}
		if current == nil {
			return nil, &perintis.Error{Status: 409, Message: "Anggota ini belum punya profil kaderisasi. Profil dibuat saat anggota melengkapinya di website kaderisasi."}
		}
		// Perintis routes read their own JSON body; it goes through the same
		// validator as the admin member page, with Indonesian 422 messages.
		var body validation.Object
		if err := perintisInput(r, &body); err != nil {
			return nil, err
		}
		validated, issues := validation.Validate("memberProfileUpdateValidator", body)
		if len(issues) > 0 {
			return nil, &perintis.Error{Status: 422, Message: profileIssue(issues[0].Field)}
		}
		data, ok := decodeInputAs[member.ProfileUpdate](w, validated)
		if !ok {
			return nil, nil
		}
		data.Password, data.ExtraData = nil, nil
		if _, err := members.UpdateProfile(r.Context(), strconv.Itoa(int(current.ID)), data); err != nil {
			legacyFailure(w, err)
			return nil, nil
		}
		profile, err := profileOf(r, target)
		return participantProfile{Profile: profile}, err
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

// profileIssue names the profile field a validation issue is about, e.g.
// "education_history.0.intake_year" → "Periksa tahun masuk pendidikan 1.".
func profileIssue(field string) string {
	labels := map[string]string{"name": "nama", "gender": "jenis kelamin", "personal_id": "nomor identitas",
		"birth_date": "tanggal lahir", "badges": "lencana", "level": "jenjang", "province_id": "provinsi domisili",
		"city_id": "kota domisili", "origin_province_id": "provinsi asal", "origin_city_id": "kota asal", "country": "negara",
		"whatsapp": "WhatsApp", "line": "ID Line", "instagram": "Instagram", "tiktok": "TikTok", "linkedin": "LinkedIn",
		"degree": "jenjang", "institution": "kampus", "faculty": "fakultas", "major": "program studi", "intake_year": "tahun masuk",
		"job_title": "posisi", "company": "perusahaan", "start_year": "tahun mulai", "end_year": "tahun selesai"}
	parts := strings.Split(field, ".")
	if len(parts) == 3 && (parts[0] == "education_history" || parts[0] == "work_history") {
		n, _ := strconv.Atoi(parts[1])
		group := map[string]string{"education_history": "pendidikan", "work_history": "pekerjaan"}[parts[0]]
		if label, ok := labels[parts[2]]; ok {
			return fmt.Sprintf("Periksa %s %s %d.", label, group, n+1)
		}
	}
	if label, ok := labels[parts[0]]; ok {
		return "Periksa isian " + label + "."
	}
	return "Data profil tidak valid."
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
