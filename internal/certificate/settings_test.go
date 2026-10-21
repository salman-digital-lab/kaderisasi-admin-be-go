package certificate

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/dbgen"
)

func TestCertificateSettingsDefaultAndSnapshot(t *testing.T) {
	activity := dbgen.Activity{ID: 9, Name: "LMD", AdditionalConfig: []byte(`{"images":["poster.webp"]}`), ActivityStart: pgtype.Date{Time: time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC), Valid: true}}
	settings, err := ParseSettings(activity, true, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !settings.IncludeScores || settings.Role != "PESERTA" || settings.EventDate != "24 Juli 2026" || settings.DocumentDate != "30 Juli 2026" {
		t.Fatalf("incorrect defaults: %+v", settings)
	}
	settings.IncludeScores = false
	settings.Venue = "Masjid Salman ITB"
	raw, _ := json.Marshal(settings)
	activity.AdditionalConfig = []byte(`{"certificate_settings":` + string(raw) + `,"images":["poster.webp"]}`)
	loaded, err := ParseSettings(activity, true, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.IncludeScores || loaded.Venue != settings.Venue {
		t.Fatalf("saved activity settings not respected: %+v", loaded)
	}
}

func TestSalmanAlwaysRequiresApprovalAndQR(t *testing.T) {
	raw := []byte(`{"scoreSheetLayout":"salman-v1","canvasWidth":794,"canvasHeight":1123,"elements":[{"id":"recipient","type":"variable-text","variable":"{{name}}","x":0,"y":0,"width":100,"height":40}]}`)
	ready := CheckReadinessValues(1, "Salman", raw)
	if ready.Ready {
		t.Fatal("Salman template published without approval and verification")
	}
}

func TestCertificateActivityNumberFormat(t *testing.T) {
	base := DefaultSettings(dbgen.Activity{}, false, time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC))
	for value, want := range map[string]bool{"": true, "1719/S.4/YPM-5000/IV-1448": true, "7/S.4/YPM-5000/XII-1448": true, "1719/S.4/YP-5000/IV-1448": false, "1719/S.4/YPM-5000/XIII-1448": false, "abc/S.4/YPM-5000/IV-1448": false, "1719/S.4/YPM-5000/IV-48": false} {
		settings := base
		settings.ActivityNumber = value
		if got := ValidSettings(settings); got != want {
			t.Errorf("ValidSettings(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestUsesActivitySettings(t *testing.T) {
	for raw, want := range map[string]bool{
		`{"elements":[{"id":"a","type":"variable-text","variable":"{{name}}"}]}`:                          false,
		`{"elements":[{"id":"a","type":"variable-text","variable":"{{organizer}}"}]}`:                     true,
		`{"elements":[{"id":"a","type":"variable-text","variable":"{{organizer}}","visible":false}]}`:     false,
		`{"elements":[{"id":"a","type":"static-text","variable":"{{organizer}}"}]}`:                       false,
		`{"elements":[{"id":"salman-code","type":"variable-text","variable":"{{certificate_code}}"}]}`:    true,
		`{"elements":[{"id":"starter-code","type":"variable-text","variable":"{{certificate_code}}"}]}`:   false,
		`{"elements":[{"id":"n","type":"variable-text","variable":"{{ certificate_activity_number }}"}]}`: true,
	} {
		if got := UsesActivitySettings([]byte(raw)); got != want {
			t.Errorf("UsesActivitySettings(%s) = %v, want %v", raw, got, want)
		}
	}
}
