//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/config"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/storage"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type httpFixture struct {
	t       *testing.T
	pool    *pgxpool.Pool
	handler http.Handler
	token   string
	adminID int32
	admins  []int32
	storage *storage.S3
}

func newHTTPFixture(t *testing.T) *httpFixture {
	t.Helper()
	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.DBSchema, "go_rewrite_") {
		t.Fatal("isolated schema required")
	}
	pool, err := database.Open(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	f := &httpFixture{t: t, pool: pool}
	t.Cleanup(func() {
		ctx := context.Background()
		if _, err := pool.Exec(ctx, "DELETE FROM tickets WHERE requester_admin_user_id=ANY($1::int[])", f.admins); err != nil {
			t.Error(err)
		}
		if _, err := pool.Exec(ctx, "DELETE FROM admin_users WHERE id=ANY($1::int[])", f.admins); err != nil {
			t.Error(err)
		}
		pool.Close()
	})
	f.storage = storage.New(c)
	uuid, err := auth.UUID()
	if err != nil {
		t.Fatal(err)
	}
	ledger := filepath.Join(os.Getenv("GO_REWRITE_ARTIFACTS"), "storage-workflow-"+uuid+".json")
	keys := []string{}
	var storageMutex sync.Mutex
	f.storage.Created = func(key string) error {
		storageMutex.Lock()
		defer storageMutex.Unlock()
		keys = append(keys, key)
		encoded, err := json.Marshal(keys)
		if err != nil {
			return err
		}
		return os.WriteFile(ledger, encoded, 0600)
	}
	t.Cleanup(func() {
		storageMutex.Lock()
		defer storageMutex.Unlock()
		for _, key := range keys {
			if err := f.storage.Delete(context.Background(), key); err != nil {
				t.Error(err)
			}
		}
		// Retain the ownership journal. The harness verifies every key with
		// HeadObject before archiving the cleanup evidence, even after failure.
	})
	app := &Server{Config: c, Pool: pool, Auth: &auth.Service{Pool: pool, Key: c.AppKey}, Storage: f.storage, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	f.handler = app.Handler()
	f.adminID = f.admin("super_admin")
	user, err := app.Auth.Build(context.Background(), mustUser(t, pool, f.adminID))
	if err != nil {
		t.Fatal(err)
	}
	f.token = user.AccessToken
	return f
}
func (f *httpFixture) admin(role string) int32 {
	f.t.Helper()
	uuid, err := auth.UUID()
	if err != nil {
		f.t.Fatal(err)
	}
	hash, err := auth.HashPassword("Fixture-password-2026!")
	if err != nil {
		f.t.Fatal(err)
	}
	var code *string
	if role != "" {
		code = &role
	}
	var id int32
	err = f.pool.QueryRow(context.Background(), `INSERT INTO admin_users(email,normalized_email,password,display_name,is_active,role_code,created_at,updated_at) VALUES($1,$1,$2,'Contract fixture',true,$3,now(),now()) RETURNING id`, uuid+"@example.test", hash, code).Scan(&id)
	if err != nil {
		f.t.Fatal(err)
	}
	f.admins = append(f.admins, id)
	return id
}
func (f *httpFixture) call(method, path string, body interface{}, token string, status int) database.Object {
	f.t.Helper()
	var data []byte
	if body != nil {
		var err error
		data, err = json.Marshal(body)
		if err != nil {
			f.t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != status {
		f.t.Fatalf("%s %s: got %d want %d: %s", method, path, w.Code, status, w.Body.String())
	}
	var result database.Object
	if status == 401 && w.Body.String() == "Unauthorized access" {
		return nil
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		f.t.Fatal(err)
	}
	return result
}
func objectData(t *testing.T, response database.Object) database.Object {
	t.Helper()
	var data database.Object
	if err := json.Unmarshal(response["data"], &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestReferenceAndDashboardContracts(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	provinceID := int32(0)
	cityID := int32(0)
	universityID := int32(0)
	defer func() {
		q := database.JSONQueries{DB: f.pool}
		for _, target := range []struct {
			table string
			id    int32
		}{{"universities", universityID}, {"cities", cityID}, {"provinces", provinceID}} {
			if target.id > 0 {
				if _, err := q.Delete(ctx, target.table, target.id); err != nil {
					t.Error(err)
				}
			}
		}
	}()
	for _, path := range []string{"/v2/provinces", "/v2/cities", "/v2/countries", "/v2/universities"} {
		f.call("GET", path, nil, "", 200)
	}
	f.call("POST", "/v2/provinces", map[string]string{"name": "Fixture province"}, "", 401)
	f.call("POST", "/v2/provinces", map[string]string{}, f.token, 422)
	provinceID = objectData(t, f.call("POST", "/v2/provinces", map[string]string{"name": "Fixture province"}, f.token, 200)).ID("id")
	cityID = objectData(t, f.call("POST", "/v2/cities", map[string]interface{}{"name": "Fixture city", "province_id": provinceID}, f.token, 200)).ID("id")
	universityID = objectData(t, f.call("POST", "/v2/universities", map[string]interface{}{"name": "Fixture university", "provinceId": provinceID}, f.token, 200)).ID("id")
	for _, target := range []struct {
		table string
		id    int32
	}{{"provinces", provinceID}, {"cities", cityID}, {"universities", universityID}} {
		path := fmt.Sprintf("/v2/%s/%d", target.table, target.id)
		got := objectData(t, f.call("GET", path, nil, "", 200))
		if got.ID("id") != target.id {
			t.Fatal("wrong reference identity")
		}
		body := map[string]interface{}{"name": "Updated fixture"}
		if target.table == "universities" {
			body["provinceId"] = provinceID
		}
		f.call("PUT", path, body, f.token, 200)
		f.call("GET", "/v2/"+target.table+"/2147483647", nil, "", 500)
	}
	f.call("GET", fmt.Sprintf("/v2/provinces/%d/cities", provinceID), nil, "", 200)
	page := objectData(t, f.call("GET", "/v2/universities?search=Updated&page=1&per_page=1", nil, "", 200))
	if !page.Has("meta") || !page.Has("data") {
		t.Fatal("pagination envelope")
	}
	for _, path := range []string{"/v2/dashboard/stats", "/v2/dashboard/profiles", "/v2/dashboard/gender", "/v2/rbac/roles", "/v2/rbac/roles/super_admin", "/v2/rbac/permissions", "/v2/rbac/requestable-targets"} {
		f.call("GET", path, nil, f.token, 200)
	}
	f.call("GET", "/v2/rbac/roles/nonexistent", nil, f.token, 404)
	for _, target := range []struct {
		table string
		id    int32
	}{{"universities", universityID}, {"cities", cityID}, {"provinces", provinceID}} {
		f.call("DELETE", fmt.Sprintf("/v2/%s/%d", target.table, target.id), nil, f.token, 200)
		f.call("DELETE", fmt.Sprintf("/v2/%s/%d", target.table, target.id), nil, f.token, 200)
	}
}
