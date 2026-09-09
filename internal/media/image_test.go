package media

import (
	"bytes"
	"errors"
	"golang.org/x/image/webp"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestImagePresets(t *testing.T) {
	for _, tt := range []struct {
		preset             Preset
		w, h, wantW, wantH int
	}{{Logo, 1600, 800, 1024, 512}, {Gallery, 3000, 1500, 2400, 1200}, {Certificate, 6000, 1000, 5000, 833}, {Logo, 100, 50, 100, 50}} {
		t.Run(string(tt.preset), func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, tt.w, tt.h))
			img.Set(0, 0, color.RGBA{R: 255, A: 255})
			var b bytes.Buffer
			if err := png.Encode(&b, img); err != nil {
				t.Fatal(err)
			}
			out, err := Optimize(b.Bytes(), tt.preset)
			if err != nil {
				t.Fatal(err)
			}
			meta, err := webp.DecodeConfig(bytes.NewReader(out))
			if err != nil {
				t.Fatal(err)
			}
			if meta.Width != tt.wantW || meta.Height != tt.wantH {
				t.Fatalf("dimensions %dx%d", meta.Width, meta.Height)
			}
		})
	}
	if _, err := Optimize([]byte("not-an-image"), Logo); !errors.Is(err, ErrInvalidImage) {
		t.Fatal("malformed image accepted")
	}
}
