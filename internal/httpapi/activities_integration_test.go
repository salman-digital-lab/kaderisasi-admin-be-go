//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/database"
	"testing"
)

func TestActivityLifecycle(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	ids := []int32{}
	defer func() {
		if _, err := f.pool.Exec(ctx, "DELETE FROM activities WHERE id=ANY($1::int[])", ids); err != nil {
			t.Error(err)
		}
	}()
	config := map[string]interface{}{"custom_selection_status": []string{"LULUS"}, "mandatory_profile_data": []interface{}{}, "additional_questionnaire": []interface{}{}, "allow_guest_registration": true}
	body := map[string]interface{}{"name": "Fixture café — 2026", "activity_start": "2026-09-10", "activity_end": "2026-09-11", "is_published": 1, "additional_config": config, "activity_type": 1, "minimum_level": 1}
	f.call("POST", "/v2/activities", map[string]interface{}{}, f.token, 422)
	one := objectData(t, f.call("POST", "/v2/activities", body, f.token, 200))
	ids = append(ids, one.ID("id"))
	two := objectData(t, f.call("POST", "/v2/activities", body, f.token, 200))
	ids = append(ids, two.ID("id"))
	if one.String("slug") != "fixture-cafe-2026" || two.String("slug") != "fixture-cafe-2026-2" {
		t.Fatalf("slug collision: %s %s", one.String("slug"), two.String("slug"))
	}
	path := fmt.Sprintf("/v2/activities/%d", one.ID("id"))
	got := objectData(t, f.call("GET", path, nil, f.token, 200))
	if got.String("activity_start") != "2026-09-10" {
		t.Fatalf("calendar date %s", got["activity_start"])
	}
	updated := objectData(t, f.call("PUT", path, map[string]interface{}{"name": "Renamed activity", "is_registration_open": true}, f.token, 200))
	if updated.String("slug") != one.String("slug") {
		t.Fatal("slug changed on rename")
	}
	f.call("GET", "/v2/activities?search=Fixture&activity_type=1&minimum_level=1&is_published=1", nil, f.token, 200)
	missing := f.call("GET", "/v2/activities/2147483647", nil, f.token, 200)
	if string(missing["data"]) != "null" {
		t.Fatal("missing activity contract")
	}
	f.call("PUT", path, map[string]interface{}{"certificate_template_id": 2147483647}, f.token, 422)
	f.call("PUT", path+"/reorder-images", map[string]interface{}{"images": []string{}}, f.token, 200)
	f.call("PUT", path+"/reorder-images", map[string]interface{}{"images": []string{"foreign-object"}}, f.token, 400)
	f.call("PUT", path+"/delete-image", map[string]interface{}{"image": "foreign-object"}, f.token, 404)
	f.call("PUT", "/v2/activities/2147483647/reorder-images", map[string]interface{}{"images": []string{}}, f.token, 404)
	var c database.Object
	json.Unmarshal(updated["additional_config"], &c)
	if !c.Bool("allow_guest_registration") || string(c["images"]) != "[]" {
		t.Fatalf("config merge %s", updated["additional_config"])
	}
}
