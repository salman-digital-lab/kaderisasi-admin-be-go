package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/club"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/media"
	"kaderisasi/admin/internal/validation"
	"net/http"
	"strings"
)

func (s *Server) uploadClubMedia(w http.ResponseWriter, r *http.Request) error {
	logo := strings.HasSuffix(r.URL.Path, "/logo")
	limit := int64(5 << 20)
	preset := media.Gallery
	kind := "media"
	if logo {
		limit = 2 << 20
		preset = media.Logo
		kind = "logo"
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
	ctx := r.Context()
	id := pathID(r, "id")
	old, err := s.queries().One(ctx, "SELECT * FROM clubs WHERE id=$1", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "CLUB_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	if !logo && len(club.ParseMedia(old["media"]).Items) >= 20 {
		return domain.Fail(409, "CLUB_MEDIA_LIMIT_REACHED")
	}
	uuid, err := auth.UUID()
	if err != nil {
		return err
	}
	key, err := media.Upload(ctx, s.Storage, body, fmt.Sprintf("club/club_%s_%d_%s", kind, id, uuid), preset)
	if errors.Is(err, media.ErrInvalidImage) {
		return domain.Fail(422, "INVALID_IMAGE")
	}
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.Storage.Delete(context.WithoutCancel(ctx), key)
		}
	}()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	locked, err := q.One(ctx, "SELECT * FROM clubs WHERE id=$1 FOR UPDATE", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "CLUB_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	change := database.Object{}
	if logo {
		change.Set("logo", key)
	} else {
		m := club.ParseMedia(locked["media"])
		if len(m.Items) >= 20 {
			return domain.Fail(409, "CLUB_MEDIA_LIMIT_REACHED")
		}
		if club.HasURL(m.Items, key) {
			return domain.Fail(409, "MEDIA_ALREADY_EXISTS")
		}
		m.Items = append(m.Items, club.Item{URL: key, Type: "image"})
		change.Set("media", m)
	}
	if _, err = q.Update(ctx, "clubs", id, change); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	if logo {
		if previous := locked.String("logo"); previous != "" && previous != key {
			_ = s.Storage.Delete(ctx, previous)
		}
		reply(w, 200, "UPLOAD_LOGO_SUCCESS", change)
	} else {
		reply(w, 200, "UPLOAD_MEDIA_SUCCESS", change)
	}
	return nil
}
func (s *Server) addYouTubeMedia(w http.ResponseWriter, r *http.Request) error {
	data, ok := input(w, r, "youtubeMediaValidator")
	if !ok {
		return nil
	}
	ctx := r.Context()
	id := pathID(r, "id")
	_, err := s.queries().One(ctx, "SELECT id FROM clubs WHERE id=$1", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "CLUB_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	video := club.YouTubeID(data.String("media_url"))
	if video == "" {
		return domain.Fail(400, "INVALID_YOUTUBE_URL")
	}
	value := "https://www.youtube.com/embed/" + video
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	locked, err := q.One(ctx, "SELECT * FROM clubs WHERE id=$1 FOR UPDATE", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "CLUB_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	m := club.ParseMedia(locked["media"])
	if len(m.Items) >= 20 {
		return domain.Fail(409, "CLUB_MEDIA_LIMIT_REACHED")
	}
	if club.HasURL(m.Items, value) {
		return domain.Fail(409, "MEDIA_ALREADY_EXISTS")
	}
	m.Items = append(m.Items, club.Item{URL: value, Type: "video", Source: "youtube"})
	change := database.Object{}
	change.Set("media", m)
	if _, err = q.Update(ctx, "clubs", id, change); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	reply(w, 200, "ADD_YOUTUBE_MEDIA_SUCCESS", change)
	return nil
}
func (s *Server) deleteClubMedia(w http.ResponseWriter, r *http.Request) error {
	data, ok := caughtValidationInput(w, r, "deleteClubMediaValidator")
	if !ok {
		return nil
	}
	ctx := r.Context()
	id := pathID(r, "id")
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := database.JSONQueries{DB: tx}
	locked, err := q.One(ctx, "SELECT * FROM clubs WHERE id=$1 FOR UPDATE", id)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "CLUB_NOT_FOUND")
	}
	if err != nil {
		return err
	}
	m := club.ParseMedia(locked["media"])
	index := -1
	for i, item := range m.Items {
		if item.URL == data.String("media_url") {
			index = i
			break
		}
	}
	if index < 0 {
		return domain.Fail(404, "MEDIA_NOT_FOUND")
	}
	removed := m.Items[index]
	m.Items = append(m.Items[:index], m.Items[index+1:]...)
	change := database.Object{}
	change.Set("media", m)
	if _, err = q.Update(ctx, "clubs", id, change); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if removed.Type == "image" {
		_ = s.Storage.Delete(ctx, removed.URL)
	}
	reply(w, 200, "DELETE_MEDIA_SUCCESS", change)
	return nil
}
