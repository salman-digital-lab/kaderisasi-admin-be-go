package media

import (
	"bytes"
	"context"
	"encoding/binary"
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
	FormUpload  Preset = "form-upload"
)

type options struct{ Width, Height, Pixels, Quality int }

var presets = map[Preset]options{Logo: {1024, 1024, 16_000_000, 90}, Gallery: {2400, 2400, 40_000_000, 85}, Certificate: {5000, 5000, 40_000_000, 90}, FormUpload: {2400, 2400, 40_000_000, 85}}
var ErrInvalidImage = errors.New("INVALID_IMAGE")
var startup sync.Once

func Optimize(data []byte, preset Preset) ([]byte, error) {
	options, ok := presets[preset]
	if !ok {
		return nil, ErrInvalidImage
	}
	if preset == FormUpload && hasAnimation(data) {
		return nil, ErrInvalidImage
	}
	startup.Do(func() { vips.Startup(&vips.Config{ConcurrencyLevel: 2}) })
	img, err := vips.NewImageFromBuffer(data)
	if err != nil {
		return nil, ErrInvalidImage
	}
	defer img.Close()
	if preset == FormUpload && (img.Pages() > 1 || (img.OriginalFormat() != vips.ImageTypeJPEG && img.OriginalFormat() != vips.ImageTypePNG && img.OriginalFormat() != vips.ImageTypeWEBP)) {
		return nil, ErrInvalidImage
	}
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

func hasAnimation(data []byte) bool {
	png := bytes.HasPrefix(data, []byte{137, 80, 78, 71, 13, 10, 26, 10})
	webp := len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	if !png && !webp {
		return false
	}
	offset := uint64(12)
	if png {
		offset = 8
	}
	for offset+8 <= uint64(len(data)) {
		length := uint64(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		kind := string(data[offset : offset+4])
		step := length + 8 + length%2
		if png {
			length = uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
			kind = string(data[offset+4 : offset+8])
			step = length + 12
		}
		if kind == "acTL" || kind == "ANIM" || kind == "ANMF" {
			return true
		}
		offset += step
	}
	return false
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
