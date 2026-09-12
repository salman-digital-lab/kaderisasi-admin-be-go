//go:build integration

package httpapi

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestActivitySetupPermissionsAndHandoff(t *testing.T) {
	for _, role := range []string{"activity_manager", "achievement_manager"} {
		t.Run(role, func(t *testing.T) {
			f := newHTTPFixture(t)
			ctx := context.Background()
			panitia := f.tokenFor(f.admin(role))
			operational := f.tokenFor(f.admin("admin"))
			// The browser sends additional_config even before the registration step.
			draftPayload := map[string]interface{}{
				"name":                 "Setup fixture",
				"description":          "",
				"club_id":              nil,
				"is_published":         0,
				"is_registration_open": false,
				"additional_config": map[string]interface{}{
					"allow_guest_registration": false,
					"custom_selection_status":  []string{},
					"mandatory_profile_data":   []interface{}{},
					"additional_questionnaire": []interface{}{},
				},
			}
			draft := objectData(t, f.call("POST", "/v2/activities", draftPayload, panitia, 200))
			id := draft.ID("id")
			defer func() {
				if _, err := f.pool.Exec(ctx, "DELETE FROM custom_forms WHERE feature_type='activity_registration' AND feature_id=$1", id); err != nil {
					t.Error(err)
				}
				if _, err := f.pool.Exec(ctx, "DELETE FROM activities WHERE id=$1", id); err != nil {
					t.Error(err)
				}
			}()
			path := fmt.Sprintf("/v2/activities/%d", id)
			row := objectData(t, f.call("GET", path, nil, panitia, 200))
			if row.Bool("is_published") || row.Bool("is_registration_open") {
				t.Fatal("new activity is not a closed draft")
			}
			f.call("PUT", path, draftPayload, panitia, 200)
			f.call("POST", "/v2/activities", map[string]interface{}{"name": "Forged", "is_published": 1}, panitia, 403)
			f.call("PUT", path, map[string]interface{}{"name": "Must roll back", "is_published": 1}, panitia, 403)
			row = objectData(t, f.call("GET", path, nil, panitia, 200))
			if row.String("name") != "Setup fixture" {
				t.Fatal("forbidden mixed update changed content")
			}
			f.call("PUT", path, map[string]interface{}{"is_registration_open": true}, panitia, 403)
			f.call("PUT", path, map[string]interface{}{"is_published": 1}, operational, 422)
			f.call("PUT", path, map[string]interface{}{"description": "<p>Ready to publish</p>", "activity_type": 0, "activity_category": 0, "minimum_level": 0}, panitia, 200)
			f.call("PUT", path, map[string]interface{}{"is_published": 1}, operational, 422)
			// Media uploads have their own storage suite; seed an owned poster reference here.
			if _, err := f.pool.Exec(ctx, `UPDATE activities SET additional_config=jsonb_set(additional_config,'{images}','["fixture-poster.webp"]') WHERE id=$1`, id); err != nil {
				t.Fatal(err)
			}
			f.call("PUT", path, map[string]interface{}{"is_published": 1}, operational, 200)
			f.call("PUT", path, map[string]interface{}{"description": "<p>Live edit by Panitia</p>"}, panitia, 200)
			f.call("PUT", path, map[string]interface{}{"description": "<p>&nbsp;</p>"}, panitia, 422)
			f.call("PUT", path+"/delete-image", map[string]interface{}{"image": "fixture-poster.webp"}, panitia, 422)
			f.call("PUT", path, map[string]interface{}{"is_published": 0}, panitia, 403)
			f.call("PUT", path, map[string]interface{}{"is_registration_open": true}, operational, 422)
			location, _ := time.LoadLocation("Asia/Jakarta")
			today := time.Now().In(location).Format("2006-01-02")
			f.call("PUT", path, map[string]interface{}{"registration_start": today, "registration_end": today}, panitia, 200)
			form := objectData(t, f.call("POST", "/v2/custom-forms", map[string]interface{}{"formName": "Setup registration", "featureType": "activity_registration", "featureId": id, "isActive": false, "formSchema": map[string]interface{}{"fields": []interface{}{}}}, panitia, 201))
			formPath := fmt.Sprintf("/v2/custom-forms/%d", form.ID("id"))
			f.call("PUT", path, map[string]interface{}{"is_registration_open": true}, operational, 422)
			f.call("PUT", formPath+"/toggle-active", nil, panitia, 200)
			f.call("PUT", path, map[string]interface{}{"is_registration_open": true}, operational, 200)
			f.call("PUT", path, map[string]interface{}{"is_registration_open": false}, panitia, 403)
			f.call("PUT", formPath, map[string]interface{}{"formName": "Live form edit"}, panitia, 200)
			f.call("PUT", formPath+"/toggle-active", nil, panitia, 400)
			f.call("PUT", formPath+"/detach-activity", nil, panitia, 400)
			f.call("DELETE", formPath, nil, panitia, 400)
			f.call("PUT", path, map[string]interface{}{"is_published": 0}, operational, 200)
			row = objectData(t, f.call("GET", path, nil, panitia, 200))
			if row.Bool("is_registration_open") {
				t.Fatal("unpublication left registration open")
			}
			f.call("PUT", formPath+"/toggle-active", nil, panitia, 200)
		})
	}
}
