//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/config"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestConcurrentLastSuperAdminProtection(t *testing.T) {
	f := newHTTPFixture(t)
	second := f.admin("super_admin")
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	service := auth.Service{Pool: f.pool, Key: c.AppKey}
	session, err := service.Build(context.Background(), mustUser(t, f.pool, second))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan int, 2)
	for _, actor := range []struct {
		id    int32
		token string
	}{{f.adminID, f.token}, {second, session.AccessToken}} {
		wg.Go(func() {
			<-start
			r := httptest.NewRequest("PUT", fmt.Sprintf("/v2/admin-users/%d", actor.id), bytes.NewBufferString(`{"role_code":null}`))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer "+actor.token)
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			results <- w.Code
		})
	}
	close(start)
	wg.Wait()
	close(results)
	counts := map[int]int{}
	for status := range results {
		counts[status]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("concurrent removal statuses: %v", counts)
	}
	var remaining int
	if err = f.pool.QueryRow(context.Background(), "SELECT count(*) FROM admin_users WHERE is_active=true AND role_code='super_admin'").Scan(&remaining); err != nil || remaining != 1 {
		t.Fatal("last Super Admin lost", remaining, err)
	}
}
