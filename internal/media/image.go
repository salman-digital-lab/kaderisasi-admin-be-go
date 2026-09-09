package media

import (
	"context"
	"errors"
	"github.com/davidbyttow/govips/v2/vips"
	"kaderisasi/admin/internal/storage"
	"math"
	"sync"
)

type Preset string

const (
	Logo        Preset = "logo"
	Gallery     Preset = "gallery"
	Certificate Preset = "certificate"
)

type options struct{ Width, Height, Pixels, Quality int }

var presets = map[Preset]options{Logo: {1024, 1024, 16_000_000, 90}, Gallery: {2400, 2400, 40_000_000, 85}, Certificate: {5000, 5000, 40_000_000, 90}}
var ErrInvalidImage = errors.New("INVALID_IMAGE")
var startup sync.Once

func Optimize(data []byte, preset Preset) ([]byte, error) {
	options, ok := presets[preset]
	if !ok {
		return nil, ErrInvalidImage
	}
	startup.Do(func() { vips.Startup(&vips.Config{ConcurrencyLevel: 2}) })
	img, err := vips.NewImageFromBuffer(data)
	if err != nil {
		return nil, ErrInvalidImage
	}
	defer img.Close()
	if img.Width() <= 0 || img.Height() <= 0 || int64(img.Width())*int64(img.Height()) > int64(options.Pixels) {
		return nil, ErrInvalidImage
	}
	if err = img.AutoRotate(); err != nil {
		return nil, ErrInvalidImage
	}
	scale := math.Min(1, math.Min(float64(options.Width)/float64(img.Width()), float64(options.Height)/float64(img.Height())))
	if scale < 1 {
		if err = img.Resize(scale, vips.KernelLanczos3); err != nil {
			return nil, ErrInvalidImage
		}
	}
	out, _, err := img.ExportWebp(&vips.WebpExportParams{Quality: options.Quality, ReductionEffort: 4, StripMetadata: true})
	if err != nil {
		return nil, ErrInvalidImage
	}
	return out, nil
}
func Upload(ctx context.Context, store storage.Store, data []byte, key string, preset Preset) (string, error) {
	contents, err := Optimize(data, preset)
	if err != nil {
		return "", err
	}
	key += ".webp"
	if err = store.Put(ctx, key, contents, "image/webp"); err != nil {
		_ = store.Delete(context.WithoutCancel(ctx), key)
		return "", err
	}
	return key, nil
}
