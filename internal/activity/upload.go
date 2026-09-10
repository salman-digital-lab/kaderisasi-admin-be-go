package activity

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/media"
	"strconv"
)

type UploadResult struct {
	Image  string   `json:"image"`
	Images []string `json:"images"`
}

func imagesFrom(raw []byte) (map[string]json.RawMessage, []string, error) {
	data := map[string]json.RawMessage{}
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, nil, err
		}
	}
	images := []string{}
	if value := data["images"]; len(value) > 0 {
		if err := json.Unmarshal(value, &images); err != nil {
			return nil, nil, err
		}
	}
	return data, images, nil
}
func (s Service) UploadImage(ctx context.Context, id string, body []byte) (*UploadResult, error) {
	activity, err := dbgen.New(s.Pool).ActivityByIdentifier(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	_, images, err := imagesFrom(activity.AdditionalConfig)
	if err != nil {
		return nil, err
	}
	if len(images) >= 8 {
		return nil, domain.Fail(409, "ACTIVITY_IMAGE_LIMIT_REACHED")
	}
	uuid, err := auth.UUID()
	if err != nil {
		return nil, err
	}
	key, err := media.Upload(ctx, s.Storage, body, "activity/"+strconv.FormatInt(int64(activity.ID), 10)+"/"+uuid, media.Gallery)
	if errors.Is(err, media.ErrInvalidImage) {
		return nil, domain.Fail(422, "INVALID_IMAGE")
	}
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.Storage.Delete(context.WithoutCancel(ctx), key)
		}
	}()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	locked, err := q.LockActivityByIdentifier(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return nil, err
	}
	data, images, err := imagesFrom(locked.AdditionalConfig)
	if err != nil {
		return nil, err
	}
	if len(images) >= 8 {
		return nil, domain.Fail(409, "ACTIVITY_IMAGE_LIMIT_REACHED")
	}
	images = append(images, key)
	data["images"], _ = json.Marshal(images)
	raw, _ := json.Marshal(data)
	if err = q.SetActivityConfig(ctx, dbgen.SetActivityConfigParams{ID: locked.ID, AdditionalConfig: raw}); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	committed = true
	return &UploadResult{Image: key, Images: images}, nil
}
