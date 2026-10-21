package linkedcourse

import (
	"errors"
	"testing"

	"kaderisasi/admin/internal/domain"
)

func status(err error) int {
	var failure *domain.Error
	if errors.As(err, &failure) {
		return failure.Status
	}
	return 0
}

func TestValidateIDs(t *testing.T) {
	if status(validateIDs(nil)) != 422 {
		t.Error("nil must be rejected")
	}
	if status(validateIDs([]int32{1, 1})) != 422 {
		t.Error("duplicates must be rejected")
	}
	if status(validateIDs([]int32{0})) != 422 {
		t.Error("non-positive ids must be rejected")
	}
	if validateIDs([]int32{}) != nil || validateIDs([]int32{3, 4}) != nil {
		t.Error("valid ids rejected")
	}
}

func TestValidateFilter(t *testing.T) {
	courses := []Course{{ID: 5}}
	cases := []struct {
		name    string
		courses []Course
		filter  Filters
		want    int
	}{
		{"no filter", nil, Filters{}, 0},
		{"bad completion", courses, Filters{Completion: "bogus"}, 422},
		{"completion without courses", nil, Filters{Completion: "completed"}, 422},
		{"unlinked course", courses, Filters{CourseID: 9}, 422},
		{"linked course", courses, Filters{CourseID: 5, Completion: "unverifiable"}, 0},
	}
	for _, c := range cases {
		if got := status(validateFilter(c.courses, c.filter)); got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
	}
}

func TestProgressLabel(t *testing.T) {
	if progressLabel("completed") != "Selesai" || progressLabel("whatever") != "Tidak tersedia" {
		t.Error("unexpected labels")
	}
}
