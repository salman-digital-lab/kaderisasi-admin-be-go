package activity

import (
	"kaderisasi/admin/internal/dbgen"
	"testing"
)

func TestActivityCourseValidation(t *testing.T) {
	for _, ids := range [][]int32{nil, {0}, {-1}, {1, 1}} {
		if validateCourseLinks(ids) == nil {
			t.Fatalf("accepted invalid links %v", ids)
		}
	}
	for _, ids := range [][]int32{{}, {2, 1}} {
		if err := validateCourseLinks(ids); err != nil {
			t.Fatal(err)
		}
	}
	links := []dbgen.LinkedActivityCoursesRow{{ID: 1}, {ID: 2}}
	for _, filter := range []RegistrationFilters{{CourseID: 3}, {CourseCompletion: "unknown"}, {CourseID: -1}} {
		if validateCourseFilter(links, filter) == nil {
			t.Fatalf("accepted invalid filter %+v", filter)
		}
	}
	if validateCourseFilter(nil, RegistrationFilters{CourseCompletion: "completed"}) == nil {
		t.Fatal("filtered an unlinked activity")
	}
	for _, status := range []string{"", "completed", "incomplete", "unverifiable"} {
		if err := validateCourseFilter(links, RegistrationFilters{CourseID: 1, CourseCompletion: status}); err != nil {
			t.Fatal(err)
		}
	}
}
