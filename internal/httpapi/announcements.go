package httpapi

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"io"
	"kaderisasi/admin/internal/announcement"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func announcementID(r *http.Request) (int32, error) {
	v, e := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if e != nil || v <= 0 {
		return 0, domain.Fail(404, "ANNOUNCEMENT_NOT_FOUND")
	}
	return int32(v), nil
}
func announcementInput(r *http.Request, v interface{}) error {
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return domain.Fail(422, "INVALID_ANNOUNCEMENT")
	}
	if d.Decode(new(struct{})) != io.EOF {
		return domain.Fail(422, "INVALID_ANNOUNCEMENT")
	}
	return nil
}
func (s *Server) registerAnnouncements() {
	q := dbgen.New(s.Pool)
	svc := announcement.Service{Pool: s.Pool}
	register := func(controller, action string, h Handler) {
		s.register(controller, action, func(w http.ResponseWriter, r *http.Request) error {
			w.Header().Set("Cache-Control", "private, no-store")
			return h(w, r)
		})
	}
	for _, action := range []string{"index", "show", "store", "update", "delete", "preview", "publish", "withdraw", "options"} {
		register("announcements_controller", action, func(w http.ResponseWriter, r *http.Request) error {
			var id int32
			var e error
			if r.PathValue("id") != "" {
				id, e = announcementID(r)
				if e != nil {
					return e
				}
			}
			switch action {
			case "index":
				before := int64(0)
				if v := r.URL.Query().Get("cursor"); v != "" {
					before, e = strconv.ParseInt(v, 10, 32)
					if e != nil || before <= 0 {
						return domain.Fail(422, "INVALID_CURSOR")
					}
				}
				rows, e := q.AnnouncementList(r.Context(), int32(before))
				if e != nil {
					return e
				}
				next := ""
				if len(rows) > 20 {
					rows = rows[:20]
					next = strconv.Itoa(int(rows[19].ID))
				}
				data := []announcement.Record{}
				for _, row := range rows {
					data = append(data, announcement.Present(row))
				}
				reply(w, 200, "GET_DATA_SUCCESS", struct {
					Items []announcement.Record `json:"items"`
					Next  string                `json:"next_cursor"`
				}{data, next})
			case "options":
				search := r.URL.Query().Get("search")
				if len(search) > 100 {
					return domain.Fail(422, "INVALID_SEARCH")
				}
				selected := []int32{}
				if raw := r.URL.Query().Get("selected"); raw != "" {
					parts := strings.Split(raw, ",")
					if len(parts) > 1000 {
						return domain.Fail(422, "INVALID_SELECTION")
					}
					for _, part := range parts {
						id, err := strconv.ParseInt(part, 10, 32)
						if err != nil || id <= 0 {
							return domain.Fail(422, "INVALID_SELECTION")
						}
						selected = append(selected, int32(id))
					}
				}
				rows, e := q.AnnouncementOptions(r.Context(), dbgen.AnnouncementOptionsParams{Search: search, Kind: r.URL.Query().Get("kind"), Selected: selected})
				if e != nil {
					return e
				}
				reply(w, 200, "GET_DATA_SUCCESS", rows)
			case "show":
				row, e := q.AnnouncementGet(r.Context(), id)
				if e != nil {
					return announcement.Missing(e)
				}
				reply(w, 200, "GET_DATA_SUCCESS", announcement.Present(row))
			case "store", "update":
				var in announcement.Input
				if e = announcementInput(r, &in); e != nil {
					return e
				}
				row, e := svc.Save(r.Context(), id, actor(r).ID, in)
				if e != nil {
					return e
				}
				status := 200
				if action == "store" {
					status = 201
				}
				reply(w, status, "SAVE_DATA_SUCCESS", row)
			case "preview":
				p, e := svc.Preview(r.Context(), id)
				if e != nil {
					return e
				}
				reply(w, 200, "GET_DATA_SUCCESS", p)
			case "publish", "delete":
				var in struct {
					Version int32 `json:"version"`
				}
				if e = announcementInput(r, &in); e != nil {
					return e
				}
				if in.Version < 1 {
					return domain.Fail(422, "INVALID_VERSION")
				}
				if action == "delete" {
					n, e := q.AnnouncementDelete(r.Context(), dbgen.AnnouncementDeleteParams{ID: id, Version: in.Version})
					if e != nil {
						return e
					}
					if n == 0 {
						return domain.Fail(409, "ANNOUNCEMENT_CHANGED")
					}
					reply(w, 200, "DELETE_DATA_SUCCESS", nil)
				} else {
					started := time.Now()
					row, e := svc.Publish(r.Context(), id, actor(r).ID, in.Version)
					if e != nil {
						s.Logger.Warn("announcement publication failed", "id", id, "duration_ms", time.Since(started).Milliseconds())
						return e
					}
					s.Logger.Info("announcement published", "id", id, "recipients", row.RecipientCount, "duration_ms", time.Since(started).Milliseconds())
					reply(w, 200, "PUBLISH_SUCCESS", row)
				}
			case "withdraw":
				row, e := q.AnnouncementWithdraw(r.Context(), id)
				if errors.Is(e, pgx.ErrNoRows) {
					return domain.Fail(409, "ANNOUNCEMENT_CHANGED")
				}
				if e != nil {
					return e
				}
				reply(w, 200, "WITHDRAW_SUCCESS", announcement.Present(row))
			}
			return nil
		})
	}
	for _, action := range []string{"index", "count", "show", "read", "readAll"} {
		register("notifications_controller", action, func(w http.ResponseWriter, r *http.Request) error {
			user := actor(r).ID
			switch action {
			case "index":
				c, e := announcement.DecodeCursor(r.URL.Query().Get("cursor"))
				if e != nil {
					return e
				}
				cutoff, e := q.AnnouncementClock(r.Context())
				if e != nil {
					return e
				}
				rows, e := q.AdminNotificationList(r.Context(), dbgen.AdminNotificationListParams{AdminUserID: &user, Unread: r.URL.Query().Get("unread") == "true", BeforeID: c.ID, BeforeTime: pgtype.Timestamptz{Time: c.Time, Valid: !c.Time.IsZero()}})
				if e != nil {
					return e
				}
				next := ""
				if len(rows) > 20 {
					rows = rows[:20]
					next = announcement.EncodeCursor(rows[19].ID, rows[19].PublishedAt.Time)
				}
				reply(w, 200, "GET_DATA_SUCCESS", struct {
					Items  []dbgen.AdminNotificationListRow `json:"items"`
					Next   string                           `json:"next_cursor"`
					Cutoff pgtype.Timestamptz               `json:"cutoff"`
				}{rows, next, cutoff})
			case "count":
				n, e := q.AdminNotificationCount(r.Context(), &user)
				if e != nil {
					return e
				}
				reply(w, 200, "GET_DATA_SUCCESS", struct {
					Unread int32 `json:"unread"`
				}{n})
			case "show", "read":
				id, e := announcementID(r)
				if e != nil {
					return e
				}
				if action == "read" {
					n, e := q.AdminNotificationRead(r.Context(), dbgen.AdminNotificationReadParams{ID: id, AdminUserID: &user})
					if e != nil {
						return e
					}
					if n == 0 {
						return domain.Fail(404, "NOTIFICATION_NOT_FOUND")
					}
				}
				row, e := q.AdminNotificationGet(r.Context(), dbgen.AdminNotificationGetParams{ID: id, AdminUserID: &user})
				if errors.Is(e, pgx.ErrNoRows) {
					return domain.Fail(404, "NOTIFICATION_NOT_FOUND")
				}
				if e != nil {
					return e
				}
				reply(w, 200, "GET_DATA_SUCCESS", row)
			case "readAll":
				var in struct {
					Cutoff time.Time `json:"cutoff"`
				}
				if e := announcementInput(r, &in); e != nil {
					return e
				}
				now, e := q.AnnouncementClock(r.Context())
				if e != nil {
					return e
				}
				if in.Cutoff.IsZero() || in.Cutoff.After(now.Time) {
					return domain.Fail(422, "INVALID_CUTOFF")
				}
				if e = q.AdminNotificationReadAll(r.Context(), dbgen.AdminNotificationReadAllParams{AdminUserID: &user, Cutoff: pgtype.Timestamptz{Time: in.Cutoff, Valid: true}}); e != nil {
					return e
				}
				reply(w, 200, "UPDATE_DATA_SUCCESS", nil)
			}
			return nil
		})
	}
}
