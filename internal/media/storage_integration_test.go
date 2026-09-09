//go:build integration

package media

import (
	"bytes"
	"context"
	"encoding/json"
	"golang.org/x/image/webp"
	"image"
	"image/png"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/storage"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRealStorageImageLifecycle(t *testing.T) {
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.DBSchema, "go_rewrite_") || c.Environment != "test" {
		t.Fatal("test environment required")
	}
	ctx := context.Background()
	store := storage.New(c)
	id, err := auth.UUID()
	if err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(os.Getenv("GO_REWRITE_ARTIFACTS"), "storage-"+id+".json")
	owned := []string{}
	store.Created = func(key string) error {
		owned = append(owned, key)
		body, _ := json.Marshal(owned)
		return os.WriteFile(ledger, body, 0600)
	}
	defer func() {
		for _, key := range owned {
			if err := store.Delete(ctx, key); err != nil {
				t.Errorf("cleanup %s: %v", key, err)
			}
		}
		// The harness retains and verifies cleanup evidence for these keys.
	}()
	var input bytes.Buffer
	if err = png.Encode(&input, image.NewRGBA(image.Rect(0, 0, 32, 16))); err != nil {
		t.Fatal(err)
	}
	key, err := Upload(ctx, store, input.Bytes(), "go-rewrite-fixtures/"+id+"/original", Logo)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := store.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	meta, err := webp.DecodeConfig(bytes.NewReader(actual))
	if err != nil || meta.Width != 32 || meta.Height != 16 {
		t.Fatal("stored image metadata", err)
	}
	copyKey := "go-rewrite-fixtures/" + id + "/copy.webp"
	if err = store.Copy(ctx, key, copyKey); err != nil {
		t.Fatal(err)
	}
	copied, err := store.Get(ctx, copyKey)
	if err != nil || !bytes.Equal(actual, copied) {
		t.Fatal("copied bytes differ", err)
	}
}
