//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCertificateTemplateLifecycleAndAssets(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	templates := []int32{}
	activities := []int32{}
	defer func() {
		for _, target := range []struct {
			table string
			ids   []int32
		}{{"activities", activities}, {"certificate_templates", templates}} {
			if _, err := f.pool.Exec(ctx, "DELETE FROM "+target.table+" WHERE id=ANY($1::int[])", target.ids); err != nil {
				t.Error(err)
			}
		}
	}()
	f.call("POST", "/v2/certificate-templates", map[string]string{"name": "Invalid direct publish", "status": "published"}, f.token, 422)
	tmpl := objectData(t, f.call("POST", "/v2/certificate-templates", map[string]string{"name": "Certificate fixture"}, f.token, 201))
	templates = append(templates, tmpl.ID("id"))
	path := fmt.Sprintf("/v2/certificate-templates/%d", tmpl.ID("id"))
	if tmpl.ID("version") != 1 || tmpl.String("status") != "draft" || nestedObject(tmpl, "readiness").Bool("ready") {
		t.Fatal("new template lifecycle")
	}
	f.call("POST", path+"/publish", map[string]int{"expectedVersion": 1}, f.token, 422)
	bg := f.imageUpload(path+"/background", nil)
	asset := f.imageUpload(path+"/assets", nil, 201)
	if bg.ID("templateVersion") != 2 || bg.ID("assetVersion") != 1 {
		t.Fatal("background versions", bg)
	}
	design := map[string]interface{}{"backgroundUrl": nil, "canvasWidth": 800, "canvasHeight": 566, "elements": []interface{}{map[string]interface{}{"id": "name", "type": "variable-text", "variable": "{{name}}", "x": 100, "y": 100, "width": 600, "height": 80}, map[string]interface{}{"id": "signature", "type": "signature", "imageUrl": asset.String("asset_key"), "x": 100, "y": 200, "width": 100, "height": 100}}}
	tmpl = objectData(t, f.call("PUT", path, map[string]interface{}{"expectedVersion": 2, "templateData": design}, f.token, 200))
	if tmpl.ID("version") != 3 || !nestedObject(tmpl, "readiness").Bool("ready") {
		t.Fatal("draft version/readiness")
	}
	copied := objectData(t, f.call("POST", path+"/duplicate", nil, f.token, 201))
	templates = append(templates, copied.ID("id"))
	copyPath := fmt.Sprintf("/v2/certificate-templates/%d", copied.ID("id"))
	if copied.ID("version") != 1 || copied.String("status") != "draft" {
		t.Fatal("copy state")
	}
	prefix := fmt.Sprintf("certificate/templates/%d/", copied.ID("id"))
	if !strings.HasPrefix(copied.String("background_image"), prefix) {
		t.Fatal("background not copied")
	}
	if _, err := f.storage.Get(ctx, copied.String("background_image")); err != nil {
		t.Fatal(err)
	}
	var copiedElements []database.Object
	copyDesign := nestedObject(copied, "template_data")
	json.Unmarshal(copyDesign["elements"], &copiedElements)
	if !strings.HasPrefix(copiedElements[1].String("imageUrl"), prefix) {
		t.Fatal("element not copied")
	}
	f.call("POST", path+"/publish", map[string]int{"expectedVersion": 3}, f.token, 200)
	f.call("PUT", path, map[string]interface{}{"expectedVersion": 4, "name": "Forbidden edit"}, f.token, 409)
	conflict := f.call("POST", path+"/archive", map[string]int{"expectedVersion": 3}, f.token, 409)
	if conflict.ID("currentVersion") != 4 || !conflict.Has("updatedAt") {
		t.Fatal("version conflict envelope")
	}
	f.call("POST", path+"/archive", map[string]int{"expectedVersion": 4}, f.token, 200)
	f.call("POST", path+"/publish", map[string]int{"expectedVersion": 5}, f.token, 200)
	activity := objectData(t, f.call("POST", "/v2/activities", map[string]interface{}{"name": "Certificate activity", "certificate_template_id": tmpl.ID("id")}, f.token, 200))
	activities = append(activities, activity.ID("id"))
	shown := objectData(t, f.call("GET", path, nil, f.token, 200))
	if shown.ID("activity_usage_count") != 1 {
		t.Fatal("usage count")
	}
	f.call("DELETE", path, nil, f.token, 409)
	for _, url := range []string{"/v2/certificate-templates?status=published&view=summary", "/v2/certificate-templates?is_active=true&search=Certificate"} {
		f.call("GET", url, nil, f.token, 200)
	}
	f.call("GET", "/v2/certificate-templates/01", nil, f.token, 400)
	f.call("GET", "/v2/certificate-templates/2147483647", nil, f.token, 404)
	// A failed real S3 copy after a successful background copy rolls back the row
	// and removes every target attempted by this duplication only.
	uuid, _ := auth.UUID()
	copiedElements[1].Set("imageUrl", prefix+"assets/missing-"+uuid+".webp")
	copyDesign.Set("elements", copiedElements)
	f.call("PUT", copyPath, map[string]interface{}{"expectedVersion": 1, "templateData": copyDesign}, f.token, 200)
	tracked := []string{}
	originalRecorder := f.storage.Created
	f.storage.Created = func(key string) error { tracked = append(tracked, key); return originalRecorder(key) }
	f.call("POST", copyPath+"/duplicate", nil, f.token, 422)
	f.storage.Created = originalRecorder
	if len(tracked) < 2 {
		t.Fatalf("copy failure did not exercise storage %v", tracked)
	}
	for _, key := range tracked {
		if _, err := f.storage.Get(ctx, key); err == nil {
			t.Errorf("failed copy leaked %s", key)
		}
	}
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM certificate_templates").Scan(&count); err != nil || count != 2 {
		t.Fatal("copy transaction rollback", count, err)
	}
	// Optimistic locking permits exactly one of two mutations with version 2.
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Go(func() {
			body, _ := json.Marshal(map[string]interface{}{"expectedVersion": 2, "description": fmt.Sprintf("competing %d", i)})
			r := httptest.NewRequest("PUT", copyPath, bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer "+f.token)
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			codes <- w.Code
		})
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("version race %v", counts)
	}
	f.call("DELETE", copyPath, nil, f.token, 200)
}
