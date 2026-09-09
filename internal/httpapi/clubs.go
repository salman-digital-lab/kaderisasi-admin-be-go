package httpapi

import (
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/club"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"slices"
	"time"
)

func (s *Server) registerClubs() {
	controller := "clubs_controller"
	s.register(controller, "index", func(w http.ResponseWriter, r *http.Request) error {
		params := r.URL.Query()
		sql := "SELECT id,name,club_type,description,short_description,logo,created_at,updated_at,start_period,end_period,is_show,is_registration_open,registration_end_date FROM clubs WHERE name ILIKE $1"
		args := []interface{}{"%" + params.Get("search") + "%"}
		if kind := params.Get("club_type"); slices.Contains([]string{"UNIT", "CLUB_KEPROFESIAN", "CLUB_BAHASA", "AVISMAN_REGIONAL"}, kind) {
			args = append(args, kind)
			sql += fmt.Sprintf(" AND club_type=$%d", len(args))
		}
		for _, pair := range [][4]string{{"visibility", "is_show", "published", "draft"}, {"registration", "is_registration_open", "open", "closed"}} {
			v := params.Get(pair[0])
			if v == pair[2] || v == pair[3] {
				args = append(args, v == pair[2])
				sql += fmt.Sprintf(" AND %s=$%d", pair[1], len(args))
			}
		}
		page, size := pageParams(r, 10, 0)
		data, err := s.queries().Paginate(r.Context(), sql+" ORDER BY is_show DESC,created_at DESC", args, page, size)
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		for _, row := range data.Data {
			database.Timestamps(row, s.Config.Location, "created_at", "updated_at")
		}
		reply(w, 200, "GET_DATA_SUCCESS", data)
		return nil
	})
	s.register(controller, "show", func(w http.ResponseWriter, r *http.Request) error {
		row, err := s.queries().One(r.Context(), "SELECT c.*,(SELECT row_to_json(f) FROM custom_forms f WHERE feature_type='club_registration' AND feature_id=c.id ORDER BY updated_at DESC,id DESC LIMIT 1) AS \"attachedCustomForm\" FROM clubs c WHERE c.id=$1", pathID(r, "id"))
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Fail(404, "CLUB_NOT_FOUND")
		}
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		if !row.Null("attachedCustomForm") {
			row.Set("attachedCustomForm", database.Timestamps(nestedObject(row, "attachedCustomForm"), s.Config.Location, "created_at", "updated_at"))
		}
		reply(w, 200, "GET_DATA_SUCCESS", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
		return nil
	})
	s.register(controller, "store", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := input(w, r, "clubValidator")
		if !ok {
			return nil
		}
		media := club.ParseMedia(data["media"])
		if club.Duplicates(media.Items) {
			return domain.Fail(409, "MEDIA_ALREADY_EXISTS")
		}
		delete(data, "logo")
		if !data.Has("club_type") {
			data.Set("club_type", "UNIT")
		}
		data.Set("media", media)
		data.Set("is_show", false)
		data.Set("is_registration_open", false)
		for _, key := range []string{"start_period", "end_period", "registration_end_date"} {
			if !data.Has(key) {
				data.Set(key, nil)
			}
		}
		row, err := s.queries().Insert(r.Context(), "clubs", data)
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		database.CreatedModel(row, data)
		reply(w, 200, "CREATE_DATA_SUCCESS", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
		return nil
	})
	s.register(controller, "update", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := input(w, r, "updateClubValidator")
		if !ok {
			return nil
		}
		if data.Has("media") && club.Duplicates(club.ParseMedia(data["media"]).Items) {
			return domain.Fail(409, "MEDIA_ALREADY_EXISTS")
		}
		delete(data, "logo")
		ctx := r.Context()
		tx, err := s.Pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		q := database.JSONQueries{DB: tx}
		id := pathID(r, "id")
		old, err := q.One(ctx, "SELECT * FROM clubs WHERE id=$1 FOR UPDATE", id)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Fail(404, "CLUB_NOT_FOUND")
		}
		if err != nil {
			return err
		}
		if data.Bool("is_registration_open") {
			count, err := q.Count(ctx, "SELECT id FROM custom_forms WHERE feature_type='club_registration' AND feature_id=$1 AND is_active=true", id)
			if err != nil {
				return err
			}
			if count == 0 {
				return domain.Fail(400, "ACTIVE_CUSTOM_FORM_REQUIRED")
			}
			end := old.String("registration_end_date")
			if data.Has("registration_end_date") {
				end = data.String("registration_end_date")
			}
			if end != "" && end < time.Now().In(s.Config.Location).Format("2006-01-02") {
				return domain.Fail(400, "REGISTRATION_END_DATE_PASSED")
			}
		}
		row, err := q.Update(ctx, "clubs", id, data)
		if err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		if err = tx.Commit(ctx); err != nil {
			return err
		}
		reply(w, 200, "UPDATE_DATA_SUCCESS", database.Timestamps(row, s.Config.Location, "created_at", "updated_at"))
		return nil
	})
	s.register(controller, "updateRegistrationInfo", func(w http.ResponseWriter, r *http.Request) error {
		data, ok := caughtValidationInput(w, r, "updateClubRegistrationInfoValidator")
		if !ok {
			return nil
		}
		info := struct {
			Info string `json:"registration_info"`
		}{data.String("registration_info")}
		change := database.Object{}
		change.Set("registration_info", info)
		if _, err := s.queries().Update(r.Context(), "clubs", pathID(r, "id"), change); err != nil {
			legacyFailure(w, paginationError(r, err))
			return nil
		}
		reply(w, 200, "REGISTRATION_INFO_UPDATED", struct {
			Info interface{} `json:"registration_info"`
		}{info})
		return nil
	})
	s.register(controller, "uploadLogo", s.uploadClubMedia)
	s.register(controller, "uploadImageMedia", s.uploadClubMedia)
	s.register(controller, "addYoutubeMedia", s.addYouTubeMedia)
	s.register(controller, "deleteMedia", s.deleteClubMedia)
}
