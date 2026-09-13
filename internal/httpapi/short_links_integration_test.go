//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/database"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/config"
)

func TestShortLinksWorkflow(t *testing.T) {
	f := newHTTPFixture(t)
	type check struct {
		Owner  string `json:"owner"`
		Path   string `json:"path"`
		Method string `json:"method"`
		Label  string `json:"label"`
		Status int    `json:"status"`
	}
	checks := []check{}
	call := func(method, path string, body interface{}, token string, status int) database.Object {
		result := f.call(method, path, body, token, status)
		label := "request"
		if status == 422 || status == 409 {
			label = "invalid-input"
		}
		if status == 404 {
			label = "missing-resource"
		}
		if strings.Contains(path, "page=bad") {
			label = "query:invalid-pagination"
		}
		checks = append(checks, check{"admin", strings.TrimPrefix(path, "/v2"), method, label, status})
		return result
	}
	defer func() {
		if !t.Failed() {
			data, err := json.Marshal(checks)
			if err != nil {
				t.Error(err)
				return
			}
			if err = os.WriteFile(filepath.Join(os.Getenv("GO_REWRITE_ARTIFACTS"), "checks.json"), data, 0600); err != nil {
				t.Error(err)
			}
		}
	}()
	ctx := context.Background()
	t.Cleanup(func() {
		_, err := f.pool.Exec(ctx, "DELETE FROM urls")
		if err != nil {
			t.Error(err)
		}
	})
	paths := []struct{ method, path string }{{"GET", "/v2/short-links"}, {"POST", "/v2/short-links"}, {"PATCH", "/v2/short-links/missing"}, {"DELETE", "/v2/short-links/missing"}}
	call("GET", "/v2/short-links?page=bad&per_page=-2", nil, f.token, 200)
	unassigned := f.tokenFor(f.admin(""))
	for _, p := range paths {
		call(p.method, p.path, nil, "", 401)
		call(p.method, p.path, nil, unassigned, 403)
	}
	for i, role := range auth.Roles() {
		token := f.tokenFor(f.admin(role.Code))
		code := fmt.Sprintf("Role%d", i)
		call("POST", "/v2/short-links", map[string]string{"code": code, "original_url": "https://example.com"}, token, 201)
		call("GET", "/v2/short-links", nil, token, 200)
		call("PATCH", "/v2/short-links/"+code, map[string]string{"original_url": "https://example.org"}, token, 200)
		call("DELETE", "/v2/short-links/"+code, nil, token, 200)
	}
	for _, code := range []string{"ab", "abcdefghijk", "health", "a/b"} {
		call("POST", "/v2/short-links", map[string]string{"code": code, "original_url": "https://example.com"}, f.token, 422)
	}
	for _, destination := range []string{"javascript:alert(1)", "https://u@example.com", "http://localhost:4000/a", "https://example.com\n"} {
		call("POST", "/v2/short-links", map[string]string{"original_url": destination}, f.token, 422)
	}
	created := objectData(t, call("POST", "/v2/short-links", map[string]string{"code": "Kajian26", "original_url": "https://example.com/a?q=1#fragment"}, f.token, 201))
	if created.String("short_url") != "http://localhost:4000/Kajian26" {
		t.Fatal("wrong base URL")
	}
	call("POST", "/v2/short-links", map[string]string{"code": "Kajian26", "original_url": "https://example.com"}, f.token, 409)
	random := objectData(t, call("POST", "/v2/short-links", map[string]string{"original_url": "https://example.com"}, f.token, 201))
	if len(random.String("code")) != 6 {
		t.Fatal("random code length")
	}
	result := objectData(t, call("GET", "/v2/short-links?search=kajian&page=1&per_page=1", nil, f.token, 200))
	if len(result["data"]) == 0 {
		t.Fatal("missing search data")
	}
	call("PATCH", "/v2/short-links/Kajian26", map[string]string{"code": "rename", "original_url": "https://example.com"}, f.token, 422)
	call("PATCH", "/v2/short-links/missing", map[string]string{"original_url": "https://example.com"}, f.token, 404)
	call("DELETE", "/v2/short-links/missing", nil, f.token, 404)
	inactive := f.admin("konselor")
	token := f.tokenFor(inactive)
	if _, err := f.pool.Exec(ctx, "UPDATE admin_users SET is_active=false WHERE id=$1", inactive); err != nil {
		t.Fatal(err)
	}
	call("GET", "/v2/short-links", nil, token, 403)

	c, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	dsn := url.URL{Scheme: "postgres", Host: fmt.Sprintf("%s:%d", c.DBHost, c.DBPort), Path: "/" + c.DBName, User: url.UserPassword(c.DBUser, c.DBPassword)}
	binary, err := filepath.Abs("../../../url-shortener/target/debug/url-shortener")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binary)
	cmd.Stdout = t.Output()
	cmd.Stderr = t.Output()
	cmd.Env = append(os.Environ(), "DATABASE_URL="+dsn.String(), "DB_SCHEMA="+c.DBSchema, "PORT=4000", "SHORT_URL_BASE_URL=http://localhost:4000")
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for i := 0; i < 600; i++ {
		resp, e := client.Get("http://localhost:4000/health")
		if e == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				break
			}
		}
		if i == 599 {
			t.Fatal("Rust readiness failed")
		}
		time.Sleep(50 * time.Millisecond)
	}
	request := func(method, path string, want int, location string) {
		t.Helper()
		req, _ := http.NewRequest(method, "http://localhost:4000"+path, nil)
		res, e := client.Do(req)
		if e != nil {
			t.Error(e)
			return
		}
		defer res.Body.Close()
		if res.StatusCode != want || res.Header.Get("Cache-Control") != "no-store" || res.Header.Get("Location") != location {
			t.Errorf("%s %s: %d %v", method, path, res.StatusCode, res.Header)
		}
	}
	request("HEAD", "/Kajian26", 307, "https://example.com/a?q=1#fragment")
	var visits int64
	if err = f.pool.QueryRow(ctx, "SELECT visit_count FROM urls WHERE id='Kajian26'").Scan(&visits); err != nil || visits != 0 {
		t.Fatalf("HEAD count: %d %v", visits, err)
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() { request("GET", "/Kajian26", 307, "https://example.com/a?q=1#fragment") })
	}
	wg.Wait()
	if err = f.pool.QueryRow(ctx, "SELECT visit_count FROM urls WHERE id='Kajian26'").Scan(&visits); err != nil || visits != 12 {
		t.Fatalf("GET count: %d %v", visits, err)
	}
	call("PATCH", "/v2/short-links/Kajian26", map[string]string{"original_url": "https://example.org/changed"}, f.token, 200)
	request("GET", "/Kajian26", 307, "https://example.org/changed")
	request("GET", "/kajian26", 404, "")
	request("POST", "/api/shorten", 404, "")
	request("GET", "/api/stats/Kajian26", 404, "")
	if _, err = f.pool.Exec(ctx, "INSERT INTO urls(id,original_url) VALUES('legacy','javascript:alert(1)')"); err != nil {
		t.Fatal(err)
	}
	request("GET", "/legacy", 500, "")
	call("DELETE", "/v2/short-links/Kajian26", nil, f.token, 200)
	request("GET", "/Kajian26", 404, "")
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "LOCK TABLE urls IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	request("GET", "/health", 503, "")
	request("GET", "/legacy", 503, "")
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	request("GET", "/health", 200, "")
}
