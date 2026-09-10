//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/certificate"
	"kaderisasi/admin/internal/database"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type certificateFixture struct {
	f                  *httpFixture
	template, activity database.Object
	ids                []int32
}

func newCertificateFixture(t *testing.T, count int) *certificateFixture {
	t.Helper()
	f := newHTTPFixture(t)
	fixture := &certificateFixture{f: f}
	// Register after f cleanup so fixture rows are removed before the pool closes.
	t.Cleanup(func() {
		ctx := context.Background()
		if fixture.activity != nil {
			if _, err := f.pool.Exec(ctx, "DELETE FROM issued_certificates WHERE activity_id=$1", fixture.activity.ID("id")); err != nil {
				t.Error(err)
			}
			if _, err := f.queries().Delete(ctx, "activities", fixture.activity.ID("id")); err != nil {
				t.Error(err)
			}
		}
		if fixture.template != nil {
			if _, err := f.queries().Delete(ctx, "certificate_templates", fixture.template.ID("id")); err != nil {
				t.Error(err)
			}
		}
	})
	design := map[string]interface{}{"backgroundUrl": nil, "canvasWidth": 800, "canvasHeight": 566, "elements": []interface{}{map[string]interface{}{"id": "participant-name", "type": "variable-text", "variable": "{{name}}", "x": 40, "y": 100, "width": 720, "height": 100}}}
	fixture.template = objectData(t, f.call("POST", "/v2/certificate-templates", map[string]interface{}{"name": "Issuance fixture", "templateData": design}, f.token, 201))
	fixture.template = objectData(t, f.call("POST", fmt.Sprintf("/v2/certificate-templates/%d/publish", fixture.template.ID("id")), map[string]int{"expectedVersion": 1}, f.token, 200))
	fixture.activity = objectData(t, f.call("POST", "/v2/activities", map[string]interface{}{"name": "Snapshot activity", "activity_start": "2026-02-28", "certificate_template_id": fixture.template.ID("id")}, f.token, 200))
	rows, err := f.pool.Query(context.Background(), "INSERT INTO activity_registrations(activity_id,status,guest_data,created_at,updated_at) SELECT $1,'LULUS KEGIATAN',jsonb_build_object('name','Peserta '||n,'email','guest-'||n||'@example.test','gender','F'),now()+n*interval '1 millisecond',now() FROM generate_series(1,$2::int) n RETURNING id", fixture.activity.ID("id"), count)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int32
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		fixture.ids = append(fixture.ids, id)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	return fixture
}
func certificateResponse(t *testing.T, result database.Object) certificate.Response {
	t.Helper()
	var response certificate.Response
	if err := json.Unmarshal(result["data"], &response); err != nil {
		t.Fatal(err)
	}
	return response
}
func TestCertificateIssuanceSnapshotsAndRevocation(t *testing.T) {
	fixture := newCertificateFixture(t, 4)
	f := fixture.f
	ctx := context.Background()
	id := fixture.ids[0]
	activityID := fixture.activity.ID("id")
	templateID := fixture.template.ID("id")
	if _, err := f.pool.Exec(ctx, "UPDATE activity_registrations SET status='TERDAFTAR' WHERE id=$1", fixture.ids[3]); err != nil {
		t.Fatal(err)
	}
	preview := certificateResponse(t, f.call("POST", "/v2/certificates/generate-single", map[string]int32{"registration_id": id}, f.token, 200))
	if preview.Certificate != nil || preview.Participant.ActivityDate != "28 Februari 2026" || preview.Activity.Start == nil || !strings.HasPrefix(*preview.Activity.Start, "2026-02-28T00:00:00.000") {
		t.Fatalf("preview date contract %+v", preview)
	}
	f.call("POST", "/v2/certificates/generate", map[string]int32{"activity_id": activityID}, f.token, 200)
	f.call("POST", "/v2/certificates/generate-single", map[string]int32{"registration_id": fixture.ids[3]}, f.token, 409)
	f.call("POST", "/v2/certificates/generate-single", map[string]int32{"registration_id": 2147483647}, f.token, 404)
	prepared := objectData(t, f.call("POST", "/v2/certificates/prepare-issuance", map[string]interface{}{"activity_id": activityID, "registration_ids": []int32{id, fixture.ids[3], 2147483647}}, f.token, 200))
	if nestedObject(prepared, "excluded").ID("missing") != 1 || nestedObject(prepared, "excluded").ID("not_eligible") != 1 {
		t.Fatal("preparation exclusions")
	}
	// Race real HTTP requests against the same PostgreSQL registration.
	type attempt struct {
		code int
		body []byte
	}
	results := make(chan attempt, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			body, _ := json.Marshal(map[string]int32{"registration_id": id})
			r := httptest.NewRequest("POST", "/v2/certificates/issue-single", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer "+f.token)
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			results <- attempt{w.Code, w.Body.Bytes()}
		})
	}
	wg.Wait()
	close(results)
	created, reused := 0, 0
	var issued certificate.Response
	for result := range results {
		if result.code == 201 {
			created++
		} else if result.code == 200 {
			reused++
		} else {
			t.Fatalf("issuance race: %d %s", result.code, result.body)
		}
		var envelope database.Object
		json.Unmarshal(result.body, &envelope)
		data := certificateResponse(t, envelope)
		if issued.Certificate != nil && issued.Certificate.ID != data.Certificate.ID {
			t.Fatal("duplicate certificate identity")
		}
		issued = data
	}
	if created != 1 || reused != 7 {
		t.Fatal("concurrent counts", created, reused)
	}
	before := issued
	certificateID := issued.Certificate.ID
	code := issued.Certificate.Code
	// Later data edits must not alter any persisted certificate snapshot.
	f.call("PUT", fmt.Sprintf("/v2/activities/%d", activityID), map[string]interface{}{"name": "Edited after issuance", "activity_start": "2027-01-01"}, f.token, 200)
	if _, err := f.pool.Exec(ctx, "UPDATE activity_registrations SET guest_data=jsonb_build_object('name','Changed guest'),status='TERDAFTAR' WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	f.call("POST", fmt.Sprintf("/v2/certificate-templates/%d/archive", templateID), map[string]int32{"expectedVersion": 2}, f.token, 200)
	after := certificateResponse(t, f.call("GET", fmt.Sprintf("/v2/certificates/%d", certificateID), nil, f.token, 200))
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("immutable snapshot changed\nbefore=%+v\nafter=%+v", before, after)
	}
	f.call("GET", "/v2/certificates/code/"+strings.ToLower(code), nil, f.token, 200)
	verified := objectData(t, f.call("GET", "/v2/certificates/verify/"+code, nil, f.token, 200))
	if !verified.Bool("valid") {
		t.Fatal("active verification")
	}
	f.call("POST", "/v2/certificates/issue-single", map[string]int32{"registration_id": id}, f.token, 200)
	f.call("POST", fmt.Sprintf("/v2/certificates/%d/revoke", certificateID), map[string]string{"reason": "Synthetic revocation"}, f.token, 200)
	f.call("POST", fmt.Sprintf("/v2/certificates/%d/revoke", certificateID), map[string]string{"reason": "Repeat revocation"}, f.token, 409)
	revoked := objectData(t, f.call("GET", "/v2/certificates/verify/"+code, nil, f.token, 200))
	if revoked.Bool("valid") {
		t.Fatal("revoked verification")
	}
	f.call("DELETE", fmt.Sprintf("/v2/activity-registrations/%d", id), nil, f.token, 409)
	f.call("POST", fmt.Sprintf("/v2/certificate-templates/%d/publish", templateID), map[string]int32{"expectedVersion": 3}, f.token, 200)
	expected := certificate.Expectation{ActivityID: float64(activityID), TemplateID: float64(templateID), TemplateVersion: 4}
	bulk := objectData(t, f.call("POST", "/v2/certificates/issue-bulk", map[string]interface{}{"registration_ids": fixture.ids, "expected": expected, "response_mode": "compact"}, f.token, 200))
	var entries []database.Object
	json.Unmarshal(bulk["results"], &entries)
	counts := map[string]int{}
	for _, entry := range entries {
		counts[entry.String("state")]++
	}
	if counts["created"] != 2 || counts["skipped"] != 2 {
		t.Fatalf("compact bulk results %s", bulk["results"])
	}
	full := objectData(t, f.call("POST", "/v2/certificates/issue-bulk", map[string]interface{}{"registration_ids": fixture.ids, "expected": expected}, f.token, 200))
	if full.ID("total_created") != 0 || full.ID("total_already_issued") != 3 || full.ID("total_skipped") != 1 {
		t.Fatalf("repeat bulk %v", full)
	}
	recipients := objectData(t, f.call("GET", fmt.Sprintf("/v2/certificates/activities/%d/recipients?state=issued_active&per_page=1&sort_order=asc", activityID), nil, f.token, 200))
	if nestedObject(recipients, "counts").ID("issued_active") != 2 || nestedObject(recipients, "counts").ID("issued_revoked") != 1 {
		t.Fatal("recipient states")
	}
	f.call("POST", "/v2/certificates/lookup", map[string]interface{}{"activity_id": activityID, "registration_ids": []int32{id, id, 2147483647}}, f.token, 200)
	for _, count := range []int{1, 20, 21, 50, 100, 200} {
		for _, indexed := range []bool{false, true} {
			params := url.Values{"activity_id": {fmt.Sprint(activityID)}, "per_page": {"100"}}
			for i := 0; i < count; i++ {
				key := "registration_ids[]"
				if indexed {
					key = fmt.Sprintf("registration_ids[%d]", i)
				}
				params.Add(key, fmt.Sprint(id))
			}
			f.call("GET", "/v2/certificates?"+params.Encode(), nil, f.token, 200)
		}
	}
	for _, query := range []string{"registration_ids[]=invalid", "registration_ids[]=-1", "registration_ids[]=1.5", "registration_ids[unexpected]=1", "page=invalid", "per_page=101"} {
		f.call("GET", "/v2/certificates?"+query, nil, f.token, 422)
	}
	tooMany := make([]int32, 201)
	for i := range tooMany {
		tooMany[i] = id
	}
	f.call("POST", "/v2/certificates/issue-bulk", map[string]interface{}{"registration_ids": tooMany}, f.token, 422)
	f.call("POST", "/v2/certificates/lookup", map[string]interface{}{"registration_ids": tooMany}, f.token, 422)
	f.call("GET", "/v2/certificates/verify/MISSING", nil, f.token, 404)
	f.call("GET", "/v2/certificates/0", nil, f.token, 400)
}
