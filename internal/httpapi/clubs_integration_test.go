//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"kaderisasi/admin/internal/database"
	"mime/multipart"
	"net/http/httptest"
	"sync"
	"testing"
)

func (f *httpFixture) imageUpload(path string, fields map[string]string, statuses ...int) database.Object {
	f.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "fixture.png")
	if err != nil {
		f.t.Fatal(err)
	}
	if err = png.Encode(part, image.NewRGBA(image.Rect(0, 0, 32, 16))); err != nil {
		f.t.Fatal(err)
	}
	for key, value := range fields {
		if err = writer.WriteField(key, value); err != nil {
			f.t.Fatal(err)
		}
	}
	writer.Close()
	r := httptest.NewRequest("POST", path, &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Authorization", "Bearer "+f.token)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	expected := 200
	if len(statuses) > 0 {
		expected = statuses[0]
	}
	if w.Code != expected {
		f.t.Fatalf("image %s: %d %s", path, w.Code, w.Body)
	}
	var result database.Object
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		f.t.Fatal(err)
	}
	return objectData(f.t, result)
}
func TestClubAndCustomFormLifecycle(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	clubs := []int32{}
	forms := []int32{}
	activities := []int32{}
	defer func() {
		for _, target := range []struct {
			table string
			ids   []int32
		}{{"custom_forms", forms}, {"activities", activities}, {"clubs", clubs}} {
			if _, err := f.pool.Exec(ctx, "DELETE FROM "+target.table+" WHERE id=ANY($1::int[])", target.ids); err != nil {
				t.Error(err)
			}
		}
	}()
	c := objectData(t, f.call("POST", "/v2/clubs", map[string]interface{}{"name": "Fixture club", "is_show": true, "is_registration_open": true, "logo": "ignored"}, f.token, 200))
	clubs = append(clubs, c.ID("id"))
	path := fmt.Sprintf("/v2/clubs/%d", c.ID("id"))
	if c.Bool("is_show") || c.Bool("is_registration_open") || c.String("logo") != "" {
		t.Fatal("new club must start as draft")
	}
	f.call("PUT", path, map[string]interface{}{"is_registration_open": true}, f.token, 400)
	f.call("PUT", path, map[string]interface{}{"is_show": true, "club_type": "CLUB_BAHASA", "start_period": "2026-01-01", "end_period": nil}, f.token, 200)
	f.call("GET", "/v2/clubs?club_type=CLUB_BAHASA&visibility=published&registration=closed", nil, f.token, 200)
	f.call("PUT", path+"/registration-info", map[string]string{"registration_info": " Fixture instructions "}, f.token, 200)
	schema := map[string]interface{}{"fields": []interface{}{map[string]interface{}{"section_name": "questions", "fields": []interface{}{map[string]interface{}{"key": "motivation", "label": "Motivasi", "required": true, "type": "text"}}}}}
	form := objectData(t, f.call("POST", "/v2/custom-forms", map[string]interface{}{"formName": "Fixture form", "formSchema": schema, "isActive": true}, f.token, 201))
	forms = append(forms, form.ID("id"))
	formPath := fmt.Sprintf("/v2/custom-forms/%d", form.ID("id"))
	for _, url := range []string{"/v2/custom-forms?search=Fixture&is_active=true", "/v2/custom-forms/unattached", formPath, "/v2/custom-forms/available-clubs", "/v2/custom-forms/available-activities"} {
		f.call("GET", url, nil, f.token, 200)
	}
	f.call("PUT", formPath+"/attach-club", map[string]int32{"clubId": c.ID("id")}, f.token, 200)
	f.call("PUT", formPath+"/attach-club", map[string]int32{"clubId": c.ID("id")}, f.token, 409)
	f.call("GET", fmt.Sprintf("/v2/custom-forms/by-feature?feature_type=club_registration&feature_id=%d", c.ID("id")), nil, f.token, 200)
	f.call("GET", "/v2/custom-forms/by-feature", nil, f.token, 400)
	f.call("POST", "/v2/custom-forms", map[string]interface{}{"formName": "Conflict", "featureType": "club_registration", "featureId": c.ID("id")}, f.token, 409)
	f.call("PUT", path, map[string]interface{}{"registration_end_date": "2000-01-01", "is_registration_open": true}, f.token, 400)
	f.call("PUT", path, map[string]interface{}{"registration_end_date": nil, "is_registration_open": true}, f.token, 200)
	f.call("PUT", formPath, map[string]interface{}{"formName": "Renamed form", "formSchema": schema}, f.token, 200)
	f.call("PUT", formPath, map[string]interface{}{"formSchema": map[string]interface{}{"fields": []interface{}{}}}, f.token, 400)
	f.call("PUT", formPath+"/toggle-active", nil, f.token, 400)
	f.call("DELETE", formPath, nil, f.token, 400)
	f.call("PUT", formPath+"/detach-club", nil, f.token, 400)
	shown := objectData(t, f.call("GET", path, nil, f.token, 200))
	if !shown.Has("attachedCustomForm") {
		t.Fatal("attached form missing")
	}
	logo := f.imageUpload(path+"/logo", nil)
	if _, err := f.storage.Get(ctx, logo.String("logo")); err != nil {
		t.Fatal(err)
	}
	replacement := f.imageUpload(path+"/logo", nil)
	if replacement.String("logo") == logo.String("logo") {
		t.Fatal("logo key reused")
	}
	if _, err := f.storage.Get(ctx, logo.String("logo")); err == nil {
		t.Fatal("old logo not removed")
	}
	uploaded := f.imageUpload(path+"/media/image", map[string]string{"media_type": "image"})
	m := nestedObject(uploaded, "media")
	var items []database.Object
	json.Unmarshal(m["items"], &items)
	if len(items) != 1 {
		t.Fatal("gallery item")
	}
	key := items[0].String("media_url")
	f.call("POST", path+"/media/youtube", map[string]string{"media_url": "https://youtu.be/abc_DEF-123", "media_type": "video", "video_source": "youtube"}, f.token, 200)
	f.call("POST", path+"/media/youtube", map[string]string{"media_url": "https://www.youtube.com/watch?v=abc_DEF-123", "media_type": "video", "video_source": "youtube"}, f.token, 409)
	f.call("POST", path+"/media/youtube", map[string]string{"media_url": "https://evil.example/watch?v=abc_DEF-123", "media_type": "video", "video_source": "youtube"}, f.token, 400)
	f.call("PUT", path+"/delete-media", map[string]string{"media_url": key}, f.token, 200)
	if _, err := f.storage.Get(ctx, key); err == nil {
		t.Fatal("deleted media remains in storage")
	}
	f.call("PUT", path+"/delete-media", map[string]string{"media_url": key}, f.token, 404)
	f.call("PUT", path, map[string]interface{}{"is_registration_open": false}, f.token, 200)
	f.call("PUT", formPath+"/toggle-active", nil, f.token, 200)
	f.call("PUT", formPath+"/toggle-active", nil, f.token, 200)
	f.call("PUT", formPath+"/detach-club", nil, f.token, 200)
	a := objectData(t, f.call("POST", "/v2/activities", map[string]string{"name": "Form activity"}, f.token, 200))
	activities = append(activities, a.ID("id"))
	f.call("PUT", formPath+"/attach-activity", map[string]int32{"activityId": a.ID("id")}, f.token, 200)
	f.call("PUT", formPath+"/attach-activity", map[string]int32{"activityId": a.ID("id")}, f.token, 400)
	f.call("PUT", formPath+"/detach-activity", nil, f.token, 200)
	f.call("DELETE", formPath, nil, f.token, 200)
	f.call("GET", formPath, nil, f.token, 404)
	// Two distinct forms race to attach to the same club. Row locking permits one.
	for i := 0; i < 2; i++ {
		row := objectData(t, f.call("POST", "/v2/custom-forms", map[string]string{"formName": fmt.Sprintf("Concurrent form %d", i)}, f.token, 201))
		forms = append(forms, row.ID("id"))
	}
	codes := make(chan int, 2)
	var wg sync.WaitGroup
	for _, id := range forms[len(forms)-2:] {
		wg.Go(func() {
			body, _ := json.Marshal(map[string]int32{"clubId": c.ID("id")})
			r := httptest.NewRequest("PUT", fmt.Sprintf("/v2/custom-forms/%d/attach-club", id), bytes.NewReader(body))
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
		t.Fatalf("concurrent attachments %v", counts)
	}
}
