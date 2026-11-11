package httpapi

import (
	"io"
	"log/slog"
	"testing"
)

func TestMemberAuditRouteIsReadOnlyAndGuarded(t *testing.T) {
	s := &Server{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	s.Handler()
	count := 0
	for _, route := range s.ImplementationInventory() {
		if route.Controller != "member_audit_controller" {
			continue
		}
		count++
		if route.Method != "GET" || route.Path != "/v2/profiles/:id/audit-logs" || !route.Implemented || !route.Auth || !route.TrustedOrigin || route.Permission != "members.audit.read" {
			t.Errorf("%+v", route)
		}
	}
	if count != 1 {
		t.Fatalf("expected one read-only audit route, found %d", count)
	}
}
