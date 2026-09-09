//go:build integration

package auth

import (
	"context"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/database"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.DBSchema, "go_rewrite_") {
		t.Fatal("isolated schema required")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	hash, err := HashPassword("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	uuid, err := UUID()
	if err != nil {
		t.Fatal(err)
	}
	email := "auth-" + uuid + "@example.test"
	var userID int32
	err = pool.QueryRow(ctx, `INSERT INTO admin_users(email,normalized_email,password,display_name,role_code,is_active,created_at,updated_at) VALUES($1,$1,$2,'Fixture','super_admin',true,now(),now()) RETURNING id`, email, hash).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := pool.Exec(ctx, "DELETE FROM admin_users WHERE id=$1", userID); err != nil {
			t.Error(err)
		}
	}()
	s := &Service{Pool: pool, Key: c.AppKey}
	session, refresh, err := s.Login(ctx, email, "fixture-password", ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if !session.IsSuperAdmin || session.User.ID != userID {
		t.Fatal("session identity or permissions")
	}
	if _, err = s.Authenticate(ctx, session.AccessToken); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Login(ctx, email, "wrong", ClientInfo{}); err == nil {
		t.Fatal("incorrect password accepted")
	}
	nextSession, next, err := s.Rotate(ctx, refresh, ClientInfo{})
	if err != nil || next == refresh {
		t.Fatal("rotation", err)
	}
	if nextSession.User.ID != userID {
		t.Fatal("rotation changed user")
	}
	if _, _, err = s.Rotate(ctx, refresh, ClientInfo{}); err == nil {
		t.Fatal("replayed token accepted")
	}
	if _, _, err = s.Rotate(ctx, next, ClientInfo{}); err == nil {
		t.Fatal("replay did not revoke family")
	}
	_, refresh, err = s.Login(ctx, email, "fixture-password", ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Logout(ctx, refresh); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.Rotate(ctx, refresh, ClientInfo{}); err == nil {
		t.Fatal("logged-out session accepted")
	}
	_, refresh, err = s.Login(ctx, email, "fixture-password", ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Go(func() { _, _, err := s.Rotate(ctx, refresh, ClientInfo{}); results <- err })
	}
	wg.Wait()
	close(results)
	successes := 0
	for result := range results {
		if result == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent refresh successes: %d", successes)
	}
	if _, err = pool.Exec(ctx, "UPDATE admin_users SET is_active=false WHERE id=$1", userID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Authenticate(ctx, session.AccessToken); err == nil {
		t.Fatal("inactive user authenticated")
	}
	s.Now = func() time.Time { return time.Now().Add(time.Hour) }
	if _, err = s.Authenticate(ctx, session.AccessToken); err == nil {
		t.Fatal("expired JWT accepted")
	}
}
