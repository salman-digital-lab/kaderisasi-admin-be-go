//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"kaderisasi/admin/internal/auth"
	"slices"
	"testing"
	"time"
)

// Ports the source participant_registration_order.spec.ts assertions using the
// harness's owned schema. Real HTTP serialization, SQL sorting and pagination
// run together, including equal timestamps and null legacy dates.
func TestParticipantRegistrationOrdering(t *testing.T) {
	for _, kind := range []string{"activity", "club registrations", "club members", "certificates"} {
		t.Run(kind, func(t *testing.T) {
			f := newHTTPFixture(t)
			ctx := context.Background()
			club := kind == "club registrations" || kind == "club members"
			var entity int32
			var table string
			if club {
				table = "clubs"
				if err := f.pool.QueryRow(ctx, "INSERT INTO clubs(name) VALUES('Registration order fixture') RETURNING id").Scan(&entity); err != nil {
					t.Fatal(err)
				}
			} else {
				table = "activities"
				uuid, _ := auth.UUID()
				if err := f.pool.QueryRow(ctx, "INSERT INTO activities(name,slug) VALUES('Registration order fixture',$1) RETURNING id", uuid).Scan(&entity); err != nil {
					t.Fatal(err)
				}
			}
			users := []int32{}
			t.Cleanup(func() {
				if _, err := f.pool.Exec(ctx, "DELETE FROM "+table+" WHERE id=$1", entity); err != nil {
					t.Error(err)
				}
				if _, err := f.pool.Exec(ctx, "DELETE FROM public_users WHERE id=ANY($1::integer[])", users); err != nil {
					t.Error(err)
				}
			})
			latest, oldest := "2026-09-09T09:30:45.000Z", "2026-09-08T09:30:44.000Z"
			ids := []int32{}
			for i, timestamp := range []*string{&latest, &oldest, &latest, nil} {
				uuid, _ := auth.UUID()
				var user, id int32
				if err := f.pool.QueryRow(ctx, "INSERT INTO public_users(email,password,created_at) VALUES($1,'fixture-only','2026-09-08T09:30:44Z') RETURNING id", uuid+"@example.test").Scan(&user); err != nil {
					t.Fatal(err)
				}
				users = append(users, user)
				if _, err := f.pool.Exec(ctx, "INSERT INTO profiles(user_id,name) VALUES($1,$2)", user, fmt.Sprintf("Person %d", i)); err != nil {
					t.Fatal(err)
				}
				updated := oldest
				if i == 1 {
					updated = latest
				}
				if club {
					if err := f.pool.QueryRow(ctx, "INSERT INTO club_registrations(club_id,member_id,status,created_at,updated_at) VALUES($1,$2,'APPROVED',CAST(CAST($3 AS text) AS timestamptz),CAST(CAST($4 AS text) AS timestamptz)) RETURNING id", entity, user, timestamp, updated).Scan(&id); err != nil {
						t.Fatal(err)
					}
				} else {
					var userID *int32
					if i%2 == 0 {
						userID = &user
					}
					guest, _ := json.Marshal(map[string]string{"name": fmt.Sprintf("Guest %d", i)})
					if err := f.pool.QueryRow(ctx, "INSERT INTO activity_registrations(activity_id,user_id,guest_data,status,created_at,updated_at) VALUES($1,$2,$3,'LULUS KEGIATAN',CAST(CAST($4 AS text) AS timestamptz),CAST(CAST($5 AS text) AS timestamptz)) RETURNING id", entity, userID, guest, timestamp, updated).Scan(&id); err != nil {
						t.Fatal(err)
					}
				}
				ids = append(ids, id)
			}
			path := fmt.Sprintf("/v2/activities/%d/registrations", entity)
			switch kind {
			case "club registrations":
				path = fmt.Sprintf("/v2/clubs/%d/registrations", entity)
			case "club members":
				path = fmt.Sprintf("/v2/clubs/%d/members", entity)
			case "certificates":
				path = fmt.Sprintf("/v2/certificates/activities/%d/recipients", entity)
			}
			check := func(query string, expected []int32) {
				t.Helper()
				body := objectData(t, f.call("GET", path+"?per_page=2&limit=2&"+query, nil, f.token, 200))
				var page struct {
					Meta struct {
						Total int `json:"total"`
					} `json:"meta"`
					Data []struct {
						ID             int32   `json:"id"`
						RegistrationID int32   `json:"registration_id"`
						CreatedAt      *string `json:"created_at"`
					} `json:"data"`
				}
				raw, _ := json.Marshal(body)
				if err := json.Unmarshal(raw, &page); err != nil {
					t.Fatal(err)
				}
				if page.Meta.Total != 4 {
					t.Fatalf("total %d", page.Meta.Total)
				}
				got := []int32{}
				for _, row := range page.Data {
					id := row.ID
					if kind == "certificates" {
						id = row.RegistrationID
					}
					got = append(got, id)
					if id == ids[3] && row.CreatedAt != nil {
						t.Fatal("missing registration date changed")
					}
					if id == ids[2] {
						if row.CreatedAt == nil {
							t.Fatal("missing registration timestamp")
						}
						parsed, err := time.Parse(time.RFC3339, *row.CreatedAt)
						if err != nil || parsed.UTC().Format("2006-01-02T15:04:05.000Z") != latest {
							t.Fatalf("timestamp %v", row.CreatedAt)
						}
					}
				}
				if !slices.Equal(got, expected) {
					t.Fatalf("%s: got %v, want %v", query, got, expected)
				}
			}
			check("page=1", []int32{ids[2], ids[0]})
			check("page=2", []int32{ids[1], ids[3]})
			check("page=1&sort_order=asc", []int32{ids[1], ids[0]})
			check("page=2&sort_order=asc", []int32{ids[2], ids[3]})
			filter := "status=LULUS+KEGIATAN"
			if club {
				filter = "status=APPROVED"
			}
			if kind == "certificates" {
				filter = "state=eligible_not_issued"
			}
			check("page=1&sort_order=asc&"+filter, []int32{ids[1], ids[0]})
		})
	}
}
