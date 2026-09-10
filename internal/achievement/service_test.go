package achievement

import (
	"kaderisasi/admin/internal/dbgen"
	"testing"
)

func TestNullableLeaderboardAccumulation(t *testing.T) {
	value := accumulated(nil, nil, nil, nil, dbgen.Achievement{Type: new(int32(2)), Score: new(int32(5))}, false)
	if value.Academic == nil || *value.Academic != "5" || value.Total == nil || *value.Total != "5" || value.Competition != nil || value.Organizational != nil {
		t.Fatalf("nullable category changes: %+v", value)
	}
	created := accumulated(nil, nil, nil, nil, dbgen.Achievement{Type: new(int32(9)), Score: new(int32(5))}, true)
	if *created.Total != "5" || *created.Academic != "0" || *created.Competition != "0" || *created.Organizational != "0" {
		t.Fatalf("new leaderboard defaults: %+v", created)
	}
	if got := addScore(new(int32(2147483647)), new(int32(1))); *got != "2147483648" {
		t.Fatal("score narrowed before PostgreSQL validation", *got)
	}
}
func TestLeaderboardMonthCompatibility(t *testing.T) {
	for _, test := range []struct {
		year, month string
		date        *string
		failure     string
	}{
		{"2026", "2", new("2026-02-01"), ""}, {"2026x", "2x", new("2026-02-01"), ""},
		{"0x7ea", "0x2", new("2026-02-01"), ""}, {"2026", "13", nil, ""}, {"2026", "0", nil, ""},
		{"2147483648", "1", nil, ""}, {"0", "1", new("0000-01-01"), ""}, {"-1", "1", new("-000001-01-01"), ""},
		{"10000", "1", new("+010000-01-01"), ""}, {"bad", "2", nil, "Invalid unit value NaN"}, {"2026", "bad", nil, "Invalid unit value NaN"},
	} {
		t.Run(test.year+"/"+test.month, func(t *testing.T) {
			got, err := monthFilters(LeaderboardFilters{Year: test.year, Month: test.month})
			if test.failure != "" {
				if err == nil || err.Error() != test.failure {
					t.Fatal("invalid unit", err)
				}
				return
			}
			if err != nil || !got.FilterMonth || (got.Month == nil) != (test.date == nil) {
				t.Fatalf("month filter %+v, %v", got, err)
			}
			if got.Month != nil && *got.Month != *test.date {
				t.Fatal(*got.Month, *test.date)
			}
		})
	}
	ignored, err := monthFilters(LeaderboardFilters{Month: "bad"})
	if err != nil || ignored.FilterMonth || ignored.FilterYear {
		t.Fatal("month without year was not ignored", err)
	}
	year, err := monthFilters(LeaderboardFilters{Year: "2026rest"})
	if err != nil || !year.FilterYear || *year.StartDate != "2026-01-01" || *year.EndDate != "2026-12-31" {
		t.Fatal("year interval", year, err)
	}
}
