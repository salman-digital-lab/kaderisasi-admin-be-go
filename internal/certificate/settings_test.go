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
