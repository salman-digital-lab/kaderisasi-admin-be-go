package httpapi

import (
	"io"
	"log/slog"
	"strings"
	"testing"

	"kaderisasi/admin/internal/auth"
)

func TestPerintisRoutesAreImplementedAndGuarded(t *testing.T) {
	s := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s.Handler()
	known := map[string]bool{}
	for _, permission := range auth.Permissions() {
		known[permission] = true
	}
	count := 0
	for _, route := range s.ImplementationInventory() {
		if !strings.HasPrefix(route.Path, "/v2/perintis/") {
			continue
		}
		count++
		if !route.Implemented || !route.Auth || !route.TrustedOrigin || !strings.HasPrefix(route.Permission, "perintis.") || !known[route.Permission] {
			t.Errorf("%s %s: %+v", route.Method, route.Path, route)
		}
	}
	if count != 14 {
		t.Fatalf("expected 14 perintis routes, found %d", count)
	}
}
