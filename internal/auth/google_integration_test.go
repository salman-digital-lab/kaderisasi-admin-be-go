//go:build integration

package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/domain"
)

func TestGoogleCryptographicLoginAndAccountLinking(t *testing.T) {
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.DBSchema, "go_rewrite_") {
		t.Fatal("owned schema required")
	}
	ctx := context.Background()
	pool, err := database.Open(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	keys := newGoogleFixture(t)
	service := Service{Pool: pool, Key: c.AppKey, GoogleClientID: "synthetic-client", Google: keys.validator}
	id, err := UUID()
	if err != nil {
		t.Fatal(err)
	}
	email := id + "@example.test"
	subject := "google-" + id
	defer func() {
		if _, err := pool.Exec(ctx, "DELETE FROM admin_users WHERE normalized_email=ANY($1::text[])", []string{email, "linked-" + email}); err != nil {
			t.Error(err)
		}
	}()
	claims := jwt.MapClaims{"email": " " + strings.ToUpper(email) + " ", "sub": subject, "name": " Fixture Google User "}
	session, refresh, err := service.GoogleLogin(ctx, keys.token(t, claims), ClientInfo{})
	if err != nil {
		t.Fatal(err)
	}
	if session.User.Email != email || session.User.Role != nil || len(session.Permissions) != 0 || refresh == "" || len(session.AuthenticationMethods) != 1 || session.AuthenticationMethods[0] != "google" {
		t.Fatal("new Google account", session.User, session.AuthenticationMethods)
	}
	claims["email"] = "changed-" + email
	repeated, _, err := service.GoogleLogin(ctx, keys.token(t, claims), ClientInfo{})
	if err != nil || repeated.User.ID != session.User.ID {
		t.Fatal("subject linking", err)
	}
	var identityEmail string
	if err = pool.QueryRow(ctx, "SELECT email FROM admin_auth_identities WHERE admin_user_id=$1", session.User.ID).Scan(&identityEmail); err != nil || identityEmail != claims["email"] {
		t.Fatal("identity email update", err)
	}
	hash, err := HashPassword("Synthetic-password")
	if err != nil {
		t.Fatal(err)
	}
	var linkedID int32
	if err = pool.QueryRow(ctx, "INSERT INTO admin_users(email,normalized_email,password,is_active,role_code,created_at,updated_at) VALUES($1,$1,$2,true,'super_admin',now(),now()) RETURNING id", "linked-"+email, hash).Scan(&linkedID); err != nil {
		t.Fatal(err)
	}
	claims["email"] = "linked-" + email
	claims["sub"] = "linked-" + subject
	linked, _, err := service.GoogleLogin(ctx, keys.token(t, claims), ClientInfo{})
	if err != nil || linked.User.ID != linkedID || !linked.IsSuperAdmin || len(linked.AuthenticationMethods) != 2 {
		t.Fatal("existing password account linking", linked.User, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE admin_users SET is_active=false WHERE id=$1", linkedID); err != nil {
		t.Fatal(err)
	}
	_, _, err = service.GoogleLogin(ctx, keys.token(t, claims), ClientInfo{})
	var failure *domain.Error
	if !errors.As(err, &failure) || failure.Message != "USER_INACTIVE" {
		t.Fatal("inactive", err)
	}
	claims["sub"] = "unverified-" + subject
	claims["email_verified"] = false
	_, _, err = service.GoogleLogin(ctx, keys.token(t, claims), ClientInfo{})
	if !errors.As(err, &failure) || failure.Message != "GOOGLE_EMAIL_NOT_VERIFIED" {
		t.Fatal("unverified", err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM admin_auth_identities WHERE provider_subject=$1", claims["sub"]).Scan(&count); err != nil || count != 0 {
		t.Fatal("unverified identity persisted", err)
	}
}
