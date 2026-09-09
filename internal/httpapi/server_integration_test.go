//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/storage"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTTPAuthAndImage(t *testing.T) {
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
	uuid, err := auth.UUID()
	if err != nil {
		t.Fatal(err)
	}
	email := uuid + "@example.test"
	hash, err := auth.HashPassword("password")
	if err != nil {
		t.Fatal(err)
	}
	var userID, activityID int32
	if err = pool.QueryRow(ctx, `INSERT INTO admin_users(email,normalized_email,password,display_name,role_code,is_active,created_at,updated_at) VALUES($1,$1,$2,'HTTP fixture','super_admin',true,now(),now()) RETURNING id`, email, hash).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(ctx, "DELETE FROM admin_users WHERE id=$1", userID)
		if err != nil {
			t.Error(err)
		}
	}()
	if err = pool.QueryRow(ctx, `INSERT INTO activities(name,slug,created_at,updated_at) VALUES('HTTP fixture',$1,now(),now()) RETURNING id`, uuid).Scan(&activityID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := pool.Exec(ctx, "DELETE FROM activities WHERE id=$1", activityID)
		if err != nil {
			t.Error(err)
		}
	}()
	store := storage.New(c)
	keys := []string{}
	ledger := filepath.Join(os.Getenv("GO_REWRITE_ARTIFACTS"), "storage-http-"+uuid+".json")
	store.Created = func(key string) error {
		keys = append(keys, key)
		data, _ := json.Marshal(keys)
		return os.WriteFile(ledger, data, 0600)
	}
	defer func() {
		for _, key := range keys {
			if err := store.Delete(ctx, key); err != nil {
				t.Error(err)
			}
		}
		// The harness verifies absence with HeadObject and archives this journal.
	}()
	app := &Server{Config: c, Pool: pool, Auth: &auth.Service{Pool: pool, Key: c.AppKey}, Storage: store, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	handler := app.Handler()
	request := func(method, path, body, token string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost:3005")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	login := request("POST", "/v2/auth/login", fmt.Sprintf(`{"email":%q,"password":"password"}`, email), "", nil)
	if login.Code != 200 {
		t.Fatalf("login %d: %s", login.Code, login.Body)
	}
	var response struct {
		Data auth.Session `json:"data"`
	}
	if err = json.Unmarshal(login.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].Path != "/v2/auth" {
		t.Fatal("refresh cookie attributes")
	}
	for _, cookie := range []*http.Cookie{nil, {Name: "token", Value: "stale"}, {Name: "other_token", Value: "stale"}} {
		w := request("GET", "/v2/auth/me/", "", response.Data.AccessToken, cookie)
		if w.Code != 200 {
			t.Fatalf("me %d: %s", w.Code, w.Body)
		}
	}
	if w := request("GET", "/v2/auth/me", "", "", nil); w.Code != 401 {
		t.Fatalf("unauthenticated access: %d", w.Code)
	}
	refreshed := request("POST", "/v2/auth/refresh", "{}", "", cookies[0])
	if refreshed.Code != 200 {
		t.Fatalf("refresh %d: %s", refreshed.Code, refreshed.Body)
	}
	if w := request("POST", "/v2/auth/refresh", "{}", "", cookies[0]); w.Code != 401 {
		t.Fatal("refresh reuse accepted")
	}
	var imageBody bytes.Buffer
	writer := multipart.NewWriter(&imageBody)
	part, err := writer.CreateFormFile("file", "fixture.png")
	if err != nil {
		t.Fatal(err)
	}
	if err = png.Encode(part, image.NewRGBA(image.Rect(0, 0, 32, 16))); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", fmt.Sprintf("/v2/activities/%d/images", activityID), &imageBody)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+response.Data.AccessToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("upload %d: %s", w.Code, w.Body)
	}
	var storedCount int
	if err = pool.QueryRow(ctx, "SELECT jsonb_array_length(additional_config::jsonb->'images') FROM activities WHERE id=$1", activityID).Scan(&storedCount); err != nil || storedCount != 1 {
		t.Fatal("image database effect", storedCount, err)
	}
}
