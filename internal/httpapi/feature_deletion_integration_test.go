//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestFeatureDeletionGuardsAndEffects(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	type check struct {
		Owner  string `json:"owner"`
		Label  string `json:"label"`
		Method string `json:"method"`
		Path   string `json:"path"`
		Status int    `json:"status"`
	}
	checks := []check{}
	call := func(label, path string, body interface{}, token string, status int) {
		f.call("DELETE", path, body, token, status)
		checks = append(checks, check{"admin", label, "DELETE", path[len("/v2"):], status})
	}
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		if directory := os.Getenv("GO_REWRITE_ARTIFACTS"); directory != "" {
			raw, err := json.Marshal(checks)
			if err == nil {
				err = os.WriteFile(filepath.Join(directory, "feature-deletion-cases.json"), raw, 0600)
			}
			if err != nil {
				t.Error(err)
			}
		}
	})
	tokens := map[string]string{"super_admin": f.token}
	for _, role := range []string{"admin", "activity_manager", "achievement_manager", "club_manager", "konselor", ""} {
		tokens[role] = f.tokenFor(f.admin(role))
	}
	for _, kind := range []string{"activities", "clubs"} {
		for _, allowedRole := range []string{"super_admin", "admin"} {
			t.Run(kind+"/"+allowedRole, func(t *testing.T) {
				name := "Deletion fixture " + kind + " " + allowedRole
				created := objectData(t, f.call("POST", "/v2/"+kind, map[string]string{"name": name, "club_type": "UNIT"}, f.token, 200))
				id := created.ID("id")
				path := fmt.Sprintf("/v2/%s/%d", kind, id)
				t.Cleanup(func() {
					if _, err := f.pool.Exec(ctx, "DELETE FROM "+kind+" WHERE id=$1", id); err != nil {
						t.Error(err)
					}
				})
				feature := "activity_registration"
				if kind == "clubs" {
					feature = "club_registration"
				}
				var formID int32
				if err := f.pool.QueryRow(ctx, "INSERT INTO custom_forms(form_name,feature_type,feature_id,is_active,created_at,updated_at) VALUES('Deletion form',$1,$2,true,now(),now()) RETURNING id", feature, id).Scan(&formID); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := f.pool.Exec(ctx, "DELETE FROM custom_forms WHERE id=$1", formID); err != nil {
						t.Error(err)
					}
				})
				if kind == "activities" {
					if _, err := f.pool.Exec(ctx, "INSERT INTO activity_registrations(activity_id,status,created_at,updated_at) VALUES($1,'REGISTERED',now(),now())", id); err != nil {
						t.Fatal(err)
					}
				} else {
					var registrationID int32
					if err := f.pool.QueryRow(ctx, "INSERT INTO club_registrations(club_id,status,created_at) VALUES($1,'APPROVED',now()) RETURNING id", id).Scan(&registrationID); err != nil {
						t.Fatal(err)
					}
					if _, err := f.pool.Exec(ctx, "INSERT INTO club_member_roles(club_registration_id,role_name) VALUES($1,'Ketua')", registrationID); err != nil {
						t.Fatal(err)
					}
					defer func() {
						var count int
						if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM club_registrations WHERE id=$1", registrationID).Scan(&count); err != nil || count != 0 {
							t.Errorf("club registration remains: %v", err)
						}
						if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM club_member_roles WHERE club_registration_id=$1", registrationID).Scan(&count); err != nil || count != 0 {
							t.Errorf("club member role remains: %v", err)
						}
					}()
					linked := objectData(t, f.call("POST", "/v2/activities", map[string]interface{}{"name": "Preserved activity", "club_id": id}, f.token, 200))
					linkedID := linked.ID("id")
					t.Cleanup(func() {
						if _, err := f.pool.Exec(ctx, "DELETE FROM activities WHERE id=$1", linkedID); err != nil {
							t.Error(err)
						}
					})
					defer func() {
						var clubID *int32
						if err := f.pool.QueryRow(ctx, "SELECT club_id FROM activities WHERE id=$1", linkedID).Scan(&clubID); err != nil || clubID != nil {
							t.Errorf("linked activity not preserved and detached: %v", err)
						}
					}()
				}
				body := map[string]string{"confirmation": name}
				call("unauthenticated", path, body, "", 401)
				for role, token := range tokens {
					if role != "admin" && role != "super_admin" {
						call("forbidden role "+role, path, body, token, 403)
					}
				}
				for _, invalid := range []interface{}{nil, map[string]string{"confirmation": ""}, map[string]string{"confirmation": "wrong"}, map[string]int{"confirmation": 123}} {
					call("invalid confirmation", path, invalid, tokens[allowedRole], 422)
				}
				call("delete as "+allowedRole, path, body, tokens[allowedRole], 200)
				call("missing resource", path, body, tokens[allowedRole], 404)
				var featureID *int32
				var active bool
				if err := f.pool.QueryRow(ctx, "SELECT feature_id,is_active FROM custom_forms WHERE id=$1", formID).Scan(&featureID, &active); err != nil || featureID != nil || active {
					t.Errorf("form not detached: %v", err)
				}
				if kind == "activities" {
					var count int
					if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM activity_registrations WHERE activity_id=$1", id).Scan(&count); err != nil || count != 0 {
						t.Errorf("registrations remain: %v", err)
					}
				}
			})
		}
	}
}

func TestFeatureDeletionPreservesCertificateHistory(t *testing.T) {
	fixture := newCertificateFixture(t, 1)
	f := fixture.f
	path := fmt.Sprintf("/v2/activities/%d", fixture.activity.ID("id"))
	f.call("POST", "/v2/certificates/issue-single", map[string]int32{"registration_id": fixture.ids[0]}, f.token, 201)
	result := f.call("DELETE", path, map[string]string{"confirmation": "Snapshot activity"}, f.token, 409)
	if result.String("message") != "ACTIVITY_HAS_CERTIFICATE_HISTORY" {
		t.Fatal(result)
	}
	f.call("GET", path, nil, f.token, 200)
}
