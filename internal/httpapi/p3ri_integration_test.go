//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"kaderisasi/admin/internal/auth"
)

func TestP3riRegistrationReview(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	uuid, err := auth.UUID()
	if err != nil {
		t.Fatal(err)
	}
	members := []int32{}
	t.Cleanup(func() {
		if _, err := f.pool.Exec(context.Background(), "DELETE FROM p3ri_registrations WHERE user_id=ANY($1::int[])", members); err != nil {
			t.Error(err)
		}
		if _, err := f.pool.Exec(context.Background(), "DELETE FROM public_users WHERE id=ANY($1::int[])", members); err != nil {
			t.Error(err)
		}
	})
	register := func(name, gender string, choices ...string) int32 {
		var user, id int32
		if err := f.pool.QueryRow(ctx, `INSERT INTO public_users(email,password,created_at,updated_at) VALUES($1,'fixture',now(),now()) RETURNING id`,
			fmt.Sprintf("%s-%d@example.test", uuid, len(members))).Scan(&user); err != nil {
			t.Fatal(err)
		}
		members = append(members, user)
		answers, _ := json.Marshal(map[string]any{
			"common":    map[string]any{"nama_lengkap": name, "jenis_kelamin": gender, "semangat": 5, "ketersediaan": map[string]string{"Ramadhan Pekan Pertama": "Bisa"}},
			"divisions": map[string]any{choices[0]: map[string]any{"pengalaman": "Panitia OSKM", "komitmen": "Siap", "q1": "Dekat Salman"}},
		})
		if err := f.pool.QueryRow(ctx, `INSERT INTO p3ri_registrations(user_id,choices,answers) VALUES($1,$2,$3) RETURNING id`, user, choices, answers).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := register("Aisyah Calon", "Perempuan", "berbuka", "website")
	register("Budi Calon", "Laki-laki", "website")

	panitia := f.tokenFor(f.admin("p3ri"))
	admin := f.tokenFor(f.admin("admin"))

	// Only Super Admin and the p3ri role reach the review.
	f.call("GET", "/v2/p3ri/registrations", nil, admin, 403)
	form := objectData(t, f.call("GET", "/v2/p3ri/form", nil, panitia, 200))
	if string(form["code"]) != `"1448h"` {
		t.Fatalf("form %s", form["code"])
	}

	list := objectData(t, f.call("GET", "/v2/p3ri/registrations?division=website", nil, panitia, 200))
	var rows []map[string]any
	_ = json.Unmarshal(list["data"], &rows)
	if len(rows) != 2 || !strings.Contains(string(list["counts"]), `"terdaftar":2`) {
		t.Fatalf("list %s", list["data"])
	}
	list = objectData(t, f.call("GET", "/v2/p3ri/registrations?division=website&choice_rank=1&search=budi", nil, f.token, 200))
	_ = json.Unmarshal(list["data"], &rows)
	if len(rows) != 1 {
		t.Fatalf("rank and search filter %s", list["data"])
	}
	list = objectData(t, f.call("GET", "/v2/p3ri/registrations?gender=Perempuan", nil, panitia, 200))
	_ = json.Unmarshal(list["data"], &rows)
	if len(rows) != 1 {
		t.Fatalf("gender filter %s", list["data"])
	}
	f.call("GET", "/v2/p3ri/registrations?status=lulus", nil, panitia, 422)

	path := fmt.Sprintf("/v2/p3ri/registrations/%d", first)
	detail := objectData(t, f.call("GET", path, nil, panitia, 200))
	if !strings.Contains(string(detail["answers"]), "Aisyah Calon") {
		t.Fatalf("detail %s", detail["answers"])
	}
	f.call("GET", "/v2/p3ri/registrations/999999999", nil, panitia, 404)

	// Diterima needs one of the applicant's choices.
	f.call("PUT", path+"/status", map[string]any{"status": "diterima"}, panitia, 422)
	f.call("PUT", path+"/status", map[string]any{"status": "diterima", "accepted_division": "sahur"}, panitia, 422)
	f.call("PUT", path+"/status", map[string]any{"status": "selesai"}, panitia, 422)
	f.call("PUT", path+"/status", map[string]any{"status": "seleksi_berkas"}, panitia, 422)
	f.call("PUT", path+"/status", map[string]any{"status": "terdaftar"}, admin, 403)
	updated := objectData(t, f.call("PUT", path+"/status", map[string]any{"status": "diterima", "accepted_division": "website", "admin_note": "Lolos wawancara"}, panitia, 200))
	if string(updated["status"]) != `"diterima"` || string(updated["accepted_division"]) != `"website"` || string(updated["status_updated_by_name"]) != `"Contract fixture"` {
		t.Fatalf("updated %v", updated)
	}
	// Moving back clears the accepted division.
	updated = objectData(t, f.call("PUT", path+"/status", map[string]any{"status": "terdaftar", "accepted_division": "website"}, panitia, 200))
	if string(updated["accepted_division"]) != "null" {
		t.Fatalf("accepted division kept: %s", updated["accepted_division"])
	}

	// Export is an xlsx.
	r := httptest.NewRequest("GET", "/v2/p3ri/registrations/export?division=website", nil)
	r.Header.Set("Authorization", "Bearer "+panitia)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Type"), "spreadsheetml") || !strings.HasPrefix(w.Body.String(), "PK") {
		t.Fatalf("export %d %s", w.Code, w.Header().Get("Content-Type"))
	}

	// Files are served only when a registration references them.
	f.call("GET", "/v2/p3ri/files?key=p3ri/1448h/1/missing.jpg", nil, panitia, 404)
	f.call("GET", "/v2/p3ri/files?key=perintis/journals/1/a.jpg", nil, panitia, 404)
}
