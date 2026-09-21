//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/xuri/excelize/v2"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMemberExportWorkflow(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	id, _ := auth.UUID()
	users := []int32{}
	defer func() {
		if _, err := f.pool.Exec(ctx, "DELETE FROM public_users WHERE id=ANY($1::int[])", users); err != nil {
			t.Error(err)
		}
	}()
	for i := 0; i < 12; i++ {
		data := objectData(t, f.call("POST", "/v2/members", map[string]string{"name": fmt.Sprintf("%s %02d", id, i), "email": fmt.Sprintf("%s-%d@example.test", id, i), "whatsapp": "00123456789"}, f.token, 201))
		var user database.Object
		if err := json.Unmarshal(data["user"], &user); err != nil {
			t.Fatal(err)
		}
		users = append(users, user.ID("id"))
	}
	if _, err := f.pool.Exec(ctx, `UPDATE profiles SET badges='["LMD"]',education_history='[{"institution":"Export University"}]' WHERE user_id=ANY($1::int[])`, users[:6]); err != nil {
		t.Fatal(err)
	}
	request := func(path, format string) []byte {
		r := httptest.NewRequest("POST", path, strings.NewReader(`{"columns":["whatsapp","email","name"],"format":"`+format+`"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+f.token)
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), "anggota."+format) {
			t.Fatalf("download %d %s", w.Code, w.Body)
		}
		return w.Body.Bytes()
	}
	for _, query := range []string{
		"search=" + id,
		"search=" + id + "&badge=LMD",
		"education_institution=Export+University&member_id=" + fmt.Sprintf("%08d", users[0]),
		"search=" + url.QueryEscape(id+"-0@example.test") + "&education_institution=Export+University&badge=LMD",
		"search=missing-" + id,
	} {
		page := objectData(t, f.call("GET", "/v2/profiles?per_page=1&"+query, nil, f.token, 200))
		var meta struct {
			Total int `json:"total"`
		}
		if err := json.Unmarshal(page["meta"], &meta); err != nil {
			t.Fatal(err)
		}
		preview := objectData(t, f.call("GET", "/v2/profiles/export/preview?"+query, nil, f.token, 200))
		if int(preview.ID("total")) != meta.Total {
			t.Fatal("preview and list disagree")
		}
		for _, format := range []string{"xlsx", "csv"} {
			body := request("/v2/profiles/export?page=2&per_page=1&"+query, format)
			var rows [][]string
			var err error
			if format == "csv" {
				rows, err = csv.NewReader(strings.NewReader(strings.TrimPrefix(string(body), "\xef\xbb\xbf"))).ReadAll()
			} else {
				book, openErr := excelize.OpenReader(bytes.NewReader(body))
				if openErr != nil {
					t.Fatal(openErr)
				}
				rows, err = book.GetRows("Anggota")
				book.Close()
			}
			if err != nil || len(rows) != meta.Total+1 {
				t.Fatalf("%s rows=%d total=%d err=%v", format, len(rows), meta.Total, err)
			}
			if len(rows[0]) != 3 || rows[0][0] != "WhatsApp" {
				t.Fatalf("column selection: %v", rows[0])
			}
			if len(rows) > 1 && rows[1][0] != "00123456789" {
				t.Fatal("lost leading zeros")
			}
		}
	}
	for _, body := range []interface{}{
		map[string]interface{}{"columns": []string{"password"}, "format": "csv"},
		map[string]interface{}{"columns": []string{}, "format": "xlsx"},
		map[string]interface{}{"columns": []string{"email"}, "format": "pdf"},
	} {
		f.call("POST", "/v2/profiles/export", body, f.token, 422)
	}
	f.call("GET", "/v2/profiles/export/preview", nil, "", 401)
	f.call("POST", "/v2/profiles/export", nil, "", 401)
	normalID := f.admin("admin")
	normalToken := f.tokenFor(normalID)
	f.call("GET", "/v2/profiles", nil, normalToken, 200)
	f.call("GET", "/v2/profiles/export/preview", nil, normalToken, 403)
	f.call("POST", "/v2/profiles/export", nil, normalToken, 403)
	if _, err := f.pool.Exec(ctx, `UPDATE admin_users SET additional_role_codes=ARRAY['super_admin']::text[] WHERE id=$1`, normalID); err != nil {
		t.Fatal(err)
	}
	f.call("GET", "/v2/profiles/export/preview", nil, normalToken, 200)
	if _, err := f.pool.Exec(ctx, `UPDATE admin_users SET additional_role_codes='{}'::text[] WHERE id=$1`, normalID); err != nil {
		t.Fatal(err)
	}
	f.call("POST", "/v2/profiles/export", nil, normalToken, 403)
}
