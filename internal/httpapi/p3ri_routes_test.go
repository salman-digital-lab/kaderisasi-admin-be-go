package httpapi

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"kaderisasi/admin/internal/auth"
)

func TestP3riRoutesAreImplementedAndGuarded(t *testing.T) {
	s := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s.Handler()
	known := map[string]bool{}
	for _, permission := range auth.Permissions() {
		known[permission] = true
	}
	count := 0
	for _, route := range s.ImplementationInventory() {
		if !strings.HasPrefix(route.Path, "/v2/p3ri/") {
			continue
		}
		count++
		if !route.Implemented || !route.Auth || !route.TrustedOrigin || !strings.HasPrefix(route.Permission, "p3ri.") || !known[route.Permission] {
			t.Errorf("%s %s: %+v", route.Method, route.Path, route)
		}
	}
	if count != 6 {
		t.Fatalf("expected 6 p3ri routes, found %d", count)
	}
}
