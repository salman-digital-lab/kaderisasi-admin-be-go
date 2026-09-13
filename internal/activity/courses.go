package activity

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"kaderisasi/admin/internal/course"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"strconv"
)

type CourseLinksInput struct {
	CourseIDs []int32 `json:"course_ids"`
}
type CourseProgress struct {
	CourseID         int32  `json:"course_id"`
	Status           string `json:"status"`
	CompletedLessons int32  `json:"completed_lessons"`
	TotalLessons     int32  `json:"total_lessons"`
}

func (s Service) CourseOptions(ctx context.Context, search string, page, size int32) (course.Page[dbgen.ActivityCourseOptionsRow], error) {
	q := dbgen.New(s.Pool)
	rows, err := q.ActivityCourseOptions(ctx, dbgen.ActivityCourseOptionsParams{Search: search, PageOffset: (page - 1) * size, PageLimit: size})
	if err != nil {
		return course.Page[dbgen.ActivityCourseOptionsRow]{}, err
	}
	total, err := q.CountActivityCourseOptions(ctx, search)
	return course.Page[dbgen.ActivityCourseOptionsRow]{Data: rows, Meta: course.Meta{Total: total, PerPage: size, CurrentPage: page, LastPage: max(1, (total+int64(size)-1)/int64(size))}}, err
}

func (s Service) LinkedCourses(ctx context.Context, id int32) ([]dbgen.LinkedActivityCoursesRow, error) {
	q := dbgen.New(s.Pool)
	if _, err := q.RegistrationActivityByIdentifier(ctx, strconv.Itoa(int(id))); err != nil {
		return nil, activityCourseError(err)
	}
	return q.LinkedActivityCourses(ctx, id)
}

func activityCourseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "ACTIVITY_NOT_FOUND")
	}
	return err
}

func validateCourseLinks(ids []int32) error {
	if ids == nil {
		return domain.Fail(422, "INVALID_ACTIVITY_COURSES")
	}
	seen := make(map[int32]bool, len(ids))
	for _, id := range ids {
		if id < 1 || seen[id] {
			return domain.Fail(422, "INVALID_ACTIVITY_COURSES")
		}
		seen[id] = true
	}
	return nil
}

func (s Service) SaveLinkedCourses(ctx context.Context, id int32, in CourseLinksInput) ([]dbgen.LinkedActivityCoursesRow, error) {
	if err := validateCourseLinks(in.CourseIDs); err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if _, err = q.LockActivityCourses(ctx, id); err != nil {
		return nil, activityCourseError(err)
	}
	count, err := q.CountExistingActivityCourses(ctx, in.CourseIDs)
	if err != nil {
		return nil, err
	}
	if count != int64(len(in.CourseIDs)) {
		return nil, domain.Fail(422, "INVALID_ACTIVITY_COURSES")
	}
	if err = q.ClearActivityCourses(ctx, id); err != nil {
		return nil, err
	}
	if err = q.InsertActivityCourses(ctx, dbgen.InsertActivityCoursesParams{ActivityID: id, CourseIds: in.CourseIDs}); err != nil {
		return nil, err
	}
	rows, err := q.LinkedActivityCourses(ctx, id)
	if err != nil {
		return nil, err
	}
	return rows, tx.Commit(ctx)
}

func validateCourseFilter(links []dbgen.LinkedActivityCoursesRow, filters RegistrationFilters) error {
	if filters.CourseID == 0 && filters.CourseCompletion == "" {
		return nil
	}
	if len(links) == 0 {
		return domain.Fail(422, "INVALID_ACTIVITY_COURSE_FILTER")
	}
	if filters.CourseCompletion != "" && filters.CourseCompletion != "completed" && filters.CourseCompletion != "incomplete" && filters.CourseCompletion != "unverifiable" {
		return domain.Fail(422, "INVALID_ACTIVITY_COURSE_FILTER")
	}
	if filters.CourseID != 0 {
		for _, link := range links {
			if link.ID == filters.CourseID {
				return nil
			}
		}
		return domain.Fail(422, "INVALID_ACTIVITY_COURSE_FILTER")
	}
	return nil
}

func registrationCourseProgress(ctx context.Context, q *dbgen.Queries, activityID int32, ids []int32) (map[int32][]CourseProgress, error) {
	result := make(map[int32][]CourseProgress, len(ids))
	for _, id := range ids {
		result[id] = []CourseProgress{}
	}
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := q.RegistrationCourseProgress(ctx, dbgen.RegistrationCourseProgressParams{ActivityID: activityID, RegistrationIds: ids})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.RegistrationID] = append(result[row.RegistrationID], CourseProgress{CourseID: row.CourseID, Status: row.Status, CompletedLessons: row.CompletedLessons, TotalLessons: row.TotalLessons})
	}
	return result, nil
}

func courseProgressLabel(status string) string {
	switch status {
	case "completed":
		return "Selesai"
	case "in_progress":
		return "Sedang berlangsung"
	case "not_started":
		return "Belum mulai"
	case "unverifiable":
		return "Tidak dapat diverifikasi"
	case "empty":
		return "Belum ada materi"
	default:
		return "Tidak tersedia"
	}
}
