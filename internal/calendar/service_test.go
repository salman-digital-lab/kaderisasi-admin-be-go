package calendar

import (
	"kaderisasi/admin/internal/dbgen"
	"testing"
	"time"
)

func TestValidateSchedule(t *testing.T) {
	start := time.Date(2026, 9, 21, 0, 0, 0, 0, WIB)
	for _, tc := range []struct {
		name  string
		in    Input
		valid bool
	}{
		{"all day", Input{Title: "Acara", StartsAt: start, EndsAt: start.Add(24 * time.Hour), AllDay: true}, true},
		{"multi day", Input{Title: "Acara", StartsAt: start, EndsAt: start.Add(72 * time.Hour), AllDay: true}, true},
		{"timed", Input{Title: "Acara", StartsAt: start.Add(time.Hour), EndsAt: start.Add(2 * time.Hour)}, true},
		{"empty title", Input{Title: "  ", StartsAt: start, EndsAt: start.Add(time.Hour)}, false},
		{"equal times", Input{Title: "Acara", StartsAt: start, EndsAt: start}, false},
		{"reversed", Input{Title: "Acara", StartsAt: start, EndsAt: start.Add(-time.Hour)}, false},
		{"non midnight", Input{Title: "Acara", StartsAt: start.Add(time.Hour), EndsAt: start.Add(24 * time.Hour), AllDay: true}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Validate(&tc.in); (err == nil) != tc.valid {
				t.Fatalf("valid=%v error=%v", tc.valid, err)
			}
		})
	}
}
func TestRange(t *testing.T) {
	for _, tc := range []struct {
		start, end string
		valid      bool
	}{
		{"2026-09-01T00:00:00+07:00", "2026-10-01T00:00:00+07:00", true},
		{"", "", false}, {"2026-09-01", "2026-10-01", false},
		{"2026-10-01T00:00:00Z", "2026-09-01T00:00:00Z", false},
		{"2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", false},
	} {
		_, _, err := Range(tc.start, tc.end)
		if (err == nil) != tc.valid {
			t.Fatalf("range %+v: %v", tc, err)
		}
	}
}
func TestPublicProjection(t *testing.T) {
	id := int32(42)
	name, slug := "Draft activity", "private-slug"
	published := false
	row := dbgen.GetCalendarEventRow{ID: 1, Title: "Public event", ActivityID: &id, ActivityName: &name, ActivitySlug: &slug, ActivityPublished: &published}
	if project(row, true).Activity != nil {
		t.Fatal("draft activity leaked")
	}
	if project(row, false).Activity == nil {
		t.Fatal("admin link missing")
	}
	published = true
	if project(row, true).Activity == nil {
		t.Fatal("published link missing")
	}
}
