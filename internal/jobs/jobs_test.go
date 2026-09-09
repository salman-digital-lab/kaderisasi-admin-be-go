package jobs

import (
	"testing"
	"time"
)

func TestCalendarCutoffs(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct{ instant, day, month string }{
		{"2028-02-29T16:59:59Z", "2028-02-29", "2028-02-01"},
		{"2028-02-29T17:00:00Z", "2028-03-01", "2028-03-01"},
		{"2026-12-31T17:00:00Z", "2027-01-01", "2027-01-01"},
		{"2026-04-30T16:59:59Z", "2026-04-30", "2026-04-01"},
	} {
		instant, err := time.Parse(time.RFC3339, test.instant)
		if err != nil {
			t.Fatal(err)
		}
		for month, want := range map[bool]string{false: test.day, true: test.month} {
			got := Cutoff(instant, jakarta, month)
			if got.Format("2006-01-02") != want || got.Hour() != 0 || got.Location() != jakarta {
				t.Fatalf("%s month=%v: %v want %s", test.instant, month, got, want)
			}
		}
	}
}
