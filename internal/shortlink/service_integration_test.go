//go:build integration && shortlinks

package shortlink

import (
	"context"
	"errors"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
	"strings"
	"testing"
)

func TestShortLinkCollisions(t *testing.T) {
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.DBSchema, "go_rewrite_short_") {
		t.Fatal("owned short-link schema required")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service := Service{Pool: pool, BaseURL: c.ShortURLBaseURL}
	for _, code := range []string{"collide", "fresh"} {
		defer pool.Exec(ctx, "DELETE FROM urls WHERE id=$1", code)
	}
	if _, err = service.Create(ctx, Input{OriginalURL: "https://example.com", Code: "collide"}); err != nil {
		t.Fatal(err)
	}
	attempts := 0
	link, err := service.create(ctx, Input{OriginalURL: "https://example.org"}, func() (string, error) {
		attempts++
		if attempts < 3 {
			return "collide", nil
		}
		return "fresh", nil
	})
	if err != nil || link.Code != "fresh" || attempts != 3 {
		t.Fatalf("collision retry: %v %d", err, attempts)
	}
	attempts = 0
	_, err = service.create(ctx, Input{OriginalURL: "https://example.org"}, func() (string, error) { attempts++; return "collide", nil })
	var failure *domain.Error
	if !errors.As(err, &failure) || failure.Status != 503 || attempts != 5 {
		t.Fatalf("retry bound: %v %d", err, attempts)
	}
}
