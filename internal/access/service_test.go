package access

import (
	"errors"
	"testing"

	"kaderisasi/admin/internal/domain"
)

func status(err error) int {
	var failure *domain.Error
	if errors.As(err, &failure) {
		return failure.Status
	}
	return 0
}

func TestResolveRolesWithoutRolesIsEmpty(t *testing.T) {
	primary, extra, err := ResolveRoles(nil, domain.Optional[[]string]{})
	if err != nil || primary != nil || len(extra) != 0 {
		t.Fatalf("got %v %v %v", primary, extra, err)
	}
}

func TestResolveRolesKeepsPrimaryFirst(t *testing.T) {
	code := "super_admin"
	primary, extra, err := ResolveRoles(&code, domain.Optional[[]string]{})
	if err != nil || primary == nil || *primary != code || len(extra) != 0 {
		t.Fatalf("got %v %v %v", primary, extra, err)
	}
}

func TestResolveRolesRejectsNullAndUnknown(t *testing.T) {
	if got := status(func() error { _, _, err := ResolveRoles(nil, domain.Optional[[]string]{Present: true}); return err }()); got != 422 {
		t.Fatalf("null roles status = %d", got)
	}
	unknown := domain.Value([]string{"no_such_role"})
	if got := status(func() error { _, _, err := ResolveRoles(nil, unknown); return err }()); got != 409 {
		t.Fatalf("unknown role status = %d", got)
	}
}
