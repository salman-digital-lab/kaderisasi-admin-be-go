package activity

import (
	"encoding/json"
	"github.com/jackc/pgx/v5/pgtype"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"testing"
	"time"
)

func TestPanitiaCannotChangePublicationOrRegistration(t *testing.T) {
	code := "activity_manager"
	permissions := auth.ForRole(&code, true)
	for _, current := range []bool{false, true} {
		row := dbgen.Activity{IsPublished: &current, IsRegistrationOpen: current}
		changed := !current
		number := json.Number("1")
		if current {
			number = "0"
		}
		for _, input := range []Input{{IsPublished: &number}, {IsRegistrationOpen: &changed}} {
			if authorizeTransition(row, input, permissions) == nil {
				t.Fatal("Panitia changed protected state")
			}
		}
		unchanged := json.Number("0")
		if current {
			unchanged = "1"
		}
		if err := authorizeTransition(row, Input{IsPublished: &unchanged, IsRegistrationOpen: &current}, permissions); err != nil {
			t.Fatal(err)
		}
		if err := authorizeTransition(row, Input{}, permissions); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPublicationAndRegistrationReadinessAreSeparate(t *testing.T) {
	description := "<p>Pelatihan untuk peserta baru.</p>"
	zero := int32(0)
	row := dbgen.Activity{Name: "Pelatihan", Description: &description, ActivityType: &zero, ActivityCategory: &zero, MinimumLevel: &zero, AdditionalConfig: []byte(`{"images":["fixture-poster.webp"]}`)}
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	r := EvaluateReadiness(row, nil, now)
	if !r.CanPublish || r.CanOpenRegistration {
		t.Fatal("information can publish before registration is prepared")
	}
	row.RegistrationStart = pgtype.Date{Time: now, Valid: true}
	row.AdditionalConfig = []byte(`{"images":[]}`)
	if EvaluateReadiness(row, nil, now).CanPublish {
		t.Fatal("poster is mandatory for publication")
	}
	row.AdditionalConfig = []byte(`{"images":["fixture-poster.webp"]}`)
	row.RegistrationEnd = pgtype.Date{Time: now, Valid: true}
	active := true
	form := &dbgen.CustomForm{IsActive: &active, FormSchema: []byte(`{"fields":[]}`)}
	if !EvaluateReadiness(row, form, now).CanOpenRegistration {
		t.Fatal("current valid registration must be ready")
	}
	if EvaluateReadiness(row, form, now.AddDate(0, 0, 1)).CanOpenRegistration {
		t.Fatal("expired registration must not open")
	}
	if EvaluateReadiness(row, form, now.AddDate(0, 0, -1)).CanOpenRegistration {
		t.Fatal("future registration must not open")
	}
	description = "<p>&nbsp;</p>"
	if EvaluateReadiness(row, form, now).CanPublish {
		t.Fatal("empty rich text is not a description")
	}
}
