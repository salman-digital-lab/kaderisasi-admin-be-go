package activity

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/storage"
	"slices"
)

type Service struct {
	Pool    *pgxpool.Pool
	Storage storage.Store
}
type ImageOrderRequest struct {
	Images []string `json:"images"`
}
type DeleteImageRequest struct {
	Image string `json:"image"`
}
type ImageList struct {
	Images []string `json:"images"`
}

func (s Service) changeImages(ctx context.Context, id int32, change func([]string) ([]string, error)) (dbgen.Activity, []string, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return dbgen.Activity{}, nil, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	row, err := q.LockActivity(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, nil, domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	if err != nil {
		return row, nil, err
	}
	config := map[string]json.RawMessage{}
	if len(row.AdditionalConfig) > 0 && string(row.AdditionalConfig) != "null" {
		if err = json.Unmarshal(row.AdditionalConfig, &config); err != nil {
			return row, nil, err
		}
	}
	images := []string{}
	if raw, ok := config["images"]; ok {
		if err = json.Unmarshal(raw, &images); err != nil {
			return row, nil, err
		}
	}
	images, err = change(images)
	if err != nil {
		return row, nil, err
	}
	config["images"], err = json.Marshal(images)
	if err != nil {
		return row, nil, err
	}
	raw, err := json.Marshal(config)
	if err != nil {
		return row, nil, err
	}
	updated, err := q.UpdateActivityConfig(ctx, dbgen.UpdateActivityConfigParams{ID: id, AdditionalConfig: raw})
	if errors.Is(err, pgx.ErrNoRows) {
		updated = row
		err = nil
	}
	if err != nil {
		return row, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return row, nil, err
	}
	return updated, images, nil
}
func (s Service) DeleteImage(ctx context.Context, id int32, data DeleteImageRequest) (ImageList, error) {
	_, images, err := s.changeImages(ctx, id, func(images []string) ([]string, error) {
		index := slices.Index(images, data.Image)
		if index < 0 {
			return nil, domain.Fail(404, "IMAGE_NOT_FOUND")
		}
		return append(images[:index], images[index+1:]...), nil
	})
	if err != nil {
		return ImageList{}, err
	}
	_ = s.Storage.Delete(ctx, data.Image)
	return ImageList{Images: images}, nil
}
func (s Service) ReorderImages(ctx context.Context, id int32, data ImageOrderRequest) (Response, error) {
	row, _, err := s.changeImages(ctx, id, func(images []string) ([]string, error) {
		unique := map[string]bool{}
		for _, key := range data.Images {
			unique[key] = true
		}
		if len(data.Images) != len(images) || len(unique) != len(images) {
			return nil, domain.Fail(400, "INVALID_IMAGE_ORDER")
		}
		for _, key := range images {
			if !unique[key] {
				return nil, domain.Fail(400, "INVALID_IMAGE_ORDER")
			}
		}
		return data.Images, nil
	})
	return View(row), err
}
