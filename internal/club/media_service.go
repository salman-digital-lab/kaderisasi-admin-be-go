package club

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/media"
)

const maximumMediaItems = 20

type UploadResponse struct {
	Logo  *string `json:"logo,omitempty"`
	Media *Media  `json:"media,omitempty"`
}

func (s Service) Upload(ctx context.Context, id string, body []byte, logo bool) (UploadResponse, error) {
	result := UploadResponse{}
	old, err := dbgen.New(s.Pool).ClubByIdentifier(ctx, id)
	if err != nil {
		return result, lookupError(err, false)
	}
	if !logo && len(ParseMedia(old.Media).Items) >= maximumMediaItems {
		return result, domain.Fail(409, "CLUB_MEDIA_LIMIT_REACHED")
	}
	uuid, err := auth.UUID()
	if err != nil {
		return result, err
	}
	kind, preset := "media", media.Gallery
	if logo {
		kind, preset = "logo", media.Logo
	}
	key, err := media.Upload(ctx, s.Storage, body, fmt.Sprintf("club/club_%s_%s_%s", kind, id, uuid), preset)
	if errors.Is(err, media.ErrInvalidImage) {
		return result, domain.Fail(422, "INVALID_IMAGE")
	}
	if err != nil {
		return result, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.Storage.Delete(context.WithoutCancel(ctx), key)
		}
	}()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	locked, err := q.LockClubByIdentifier(ctx, id)
	if err != nil {
		return result, lookupError(err, true)
	}
	if logo {
		_, err = q.UpdateClubLogo(ctx, dbgen.UpdateClubLogoParams{ID: locked.ID, Logo: key})
		result.Logo = &key
	} else {
		m := ParseMedia(locked.Media)
		if len(m.Items) >= maximumMediaItems {
			return result, domain.Fail(409, "CLUB_MEDIA_LIMIT_REACHED")
		}
		if HasURL(m.Items, key) {
			return result, domain.Fail(409, "MEDIA_ALREADY_EXISTS")
		}
		m.Items = append(m.Items, Item{URL: key, Type: "image"})
		var raw []byte
		raw, err = json.Marshal(m)
		if err == nil {
			_, err = q.UpdateClubMedia(ctx, dbgen.UpdateClubMediaParams{ID: locked.ID, Media: raw})
		}
		result.Media = &m
	}
	if err != nil {
		return UploadResponse{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return UploadResponse{}, err
	}
	committed = true
	if logo && locked.Logo != nil && *locked.Logo != "" && *locked.Logo != key {
		_ = s.Storage.Delete(ctx, *locked.Logo)
	}
	return result, nil
}
func (s Service) changeMedia(ctx context.Context, id string, change func(*Media) error) (MediaResponse, error) {
	result := MediaResponse{}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	locked, err := q.LockClubByIdentifier(ctx, id)
	if err != nil {
		return result, lookupError(err, true)
	}
	result.Media = ParseMedia(locked.Media)
	if err = change(&result.Media); err != nil {
		return MediaResponse{}, err
	}
	raw, err := json.Marshal(result.Media)
	if err != nil {
		return MediaResponse{}, err
	}
	if _, err = q.UpdateClubMedia(ctx, dbgen.UpdateClubMediaParams{ID: locked.ID, Media: raw}); err != nil {
		return MediaResponse{}, err
	}
	return result, tx.Commit(ctx)
}
func (s Service) AddYouTube(ctx context.Context, id string, data MediaRequest) (MediaResponse, error) {
	if _, err := dbgen.New(s.Pool).ClubByIdentifier(ctx, id); err != nil {
		return MediaResponse{}, lookupError(err, false)
	}
	video := YouTubeID(data.URL)
	if video == "" {
		return MediaResponse{}, domain.Fail(400, "INVALID_YOUTUBE_URL")
	}
	value := "https://www.youtube.com/embed/" + video
	return s.changeMedia(ctx, id, func(m *Media) error {
		if len(m.Items) >= maximumMediaItems {
			return domain.Fail(409, "CLUB_MEDIA_LIMIT_REACHED")
		}
		if HasURL(m.Items, value) {
			return domain.Fail(409, "MEDIA_ALREADY_EXISTS")
		}
		m.Items = append(m.Items, Item{URL: value, Type: "video", Source: "youtube"})
		return nil
	})
}
func (s Service) DeleteMedia(ctx context.Context, id string, data MediaRequest) (MediaResponse, error) {
	var removed Item
	result, err := s.changeMedia(ctx, id, func(m *Media) error {
		for i, item := range m.Items {
			if item.URL == data.URL {
				removed = item
				m.Items = append(m.Items[:i], m.Items[i+1:]...)
				return nil
			}
		}
		return domain.Fail(404, "MEDIA_NOT_FOUND")
	})
	if err == nil && removed.Type == "image" {
		_ = s.Storage.Delete(ctx, removed.URL)
	}
	return result, err
}
