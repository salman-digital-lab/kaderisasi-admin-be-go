package linkedcourse

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/course"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/export"
	"strconv"
)

type Service struct{ Pool *pgxpool.Pool }
type Scope struct {
	Kind string
	ID   int32
}
type Course struct {
	ID           int32  `json:"id"`
	Title        string `json:"title"`
	Status       string `json:"status"`
	Summary      string `json:"summary"`
	MinimumLevel int32  `json:"minimum_level"`
	LessonCount  int32  `json:"lesson_count"`
}
type LinksInput struct {
	CourseIDs []int32 `json:"course_ids"`
}
type Progress struct {
	CourseID         int32  `json:"course_id"`
	Status           string `json:"status"`
	CompletedLessons int32  `json:"completed_lessons"`
	TotalLessons     int32  `json:"total_lessons"`
}
type Person struct {
	dbgen.ListLinkedCoursePeopleRow
	CourseProgress []Progress `json:"course_progress"`
}
type Filters struct {
	Search, Status, Completion string
	CourseID, Page, Size       int32
}
type Result struct {
	course.Page[Person]
	Courses []Course `json:"courses"`
}

func links(ctx context.Context, q *dbgen.Queries, scope Scope) ([]Course, error) {
	result := []Course{}
	if scope.Kind == "activity" {
		if _, err := q.RegistrationActivityByIdentifier(ctx, strconv.Itoa(int(scope.ID))); err != nil {
			return nil, missing(err, "ACTIVITY_NOT_FOUND")
		}
		rows, err := q.LinkedActivityCourses(ctx, scope.ID)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			result = append(result, Course{r.ID, r.Title, r.Status, r.Summary, r.MinimumLevel, r.LessonCount})
		}
	} else if scope.Kind == "club" {
		if _, err := q.ClubByIdentifier(ctx, strconv.Itoa(int(scope.ID))); err != nil {
			return nil, missing(err, "CLUB_NOT_FOUND")
		}
		rows, err := q.LinkedClubCourses(ctx, scope.ID)
		if err != nil {
			return nil, err
		}
		for _, r := range rows {
			result = append(result, Course{r.ID, r.Title, r.Status, r.Summary, r.MinimumLevel, r.LessonCount})
		}
	} else {
		return nil, domain.Fail(404, "NOT_FOUND")
	}
	return result, nil
}
func missing(err error, message string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, message)
	}
	return err
}
func (s Service) Links(ctx context.Context, scope Scope) ([]Course, error) {
	return links(ctx, dbgen.New(s.Pool), scope)
}
func validateIDs(ids []int32) error {
	if ids == nil {
		return domain.Fail(422, "INVALID_CLUB_COURSES")
	}
	seen := map[int32]bool{}
	for _, id := range ids {
		if id < 1 || seen[id] {
			return domain.Fail(422, "INVALID_CLUB_COURSES")
		}
		seen[id] = true
	}
	return nil
}
func (s Service) SaveClubLinks(ctx context.Context, id int32, in LinksInput) ([]Course, error) {
	if err := validateIDs(in.CourseIDs); err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	if _, err = q.LockClubCourses(ctx, id); err != nil {
		return nil, missing(err, "CLUB_NOT_FOUND")
	}
	n, err := q.CountExistingClubCourses(ctx, in.CourseIDs)
	if err != nil {
		return nil, err
	}
	if n != int64(len(in.CourseIDs)) {
		return nil, domain.Fail(422, "INVALID_CLUB_COURSES")
	}
	if err = q.ClearClubCourses(ctx, id); err != nil {
		return nil, err
	}
	if err = q.InsertClubCourses(ctx, dbgen.InsertClubCoursesParams{ClubID: id, CourseIds: in.CourseIDs}); err != nil {
		return nil, err
	}
	rows, err := links(ctx, q, Scope{"club", id})
	if err != nil {
		return nil, err
	}
	return rows, tx.Commit(ctx)
}
func validateFilter(courses []Course, f Filters) error {
	if f.Completion != "" && f.Completion != "completed" && f.Completion != "incomplete" && f.Completion != "unverifiable" {
		return domain.Fail(422, "INVALID_COURSE_PROGRESS_FILTER")
	}
	if f.CourseID == 0 && f.Completion == "" {
		return nil
	}
	if len(courses) == 0 {
		return domain.Fail(422, "INVALID_COURSE_PROGRESS_FILTER")
	}
	if f.CourseID != 0 {
		for _, c := range courses {
			if c.ID == f.CourseID {
				return nil
			}
		}
		return domain.Fail(422, "INVALID_COURSE_PROGRESS_FILTER")
	}
	return nil
}
func read(ctx context.Context, q *dbgen.Queries, scope Scope, f Filters) (Result, error) {
	result := Result{Page: course.Page[Person]{Data: []Person{}}}
	courses, err := links(ctx, q, scope)
	if err != nil {
		return result, err
	}
	result.Courses = courses
	if err = validateFilter(courses, f); err != nil {
		return result, err
	}
	n, err := q.CountLinkedCoursePeople(ctx, dbgen.CountLinkedCoursePeopleParams{Kind: scope.Kind, OwnerID: scope.ID, Search: f.Search, RegistrationStatus: f.Status, Completion: f.Completion, CourseID: f.CourseID})
	if err != nil {
		return result, err
	}
	result.Meta = course.Meta{Total: n, PerPage: f.Size, CurrentPage: f.Page, LastPage: max(1, (n+int64(f.Size)-1)/int64(f.Size))}
	rows, err := q.ListLinkedCoursePeople(ctx, dbgen.ListLinkedCoursePeopleParams{Kind: scope.Kind, OwnerID: scope.ID, Search: f.Search, RegistrationStatus: f.Status, Completion: f.Completion, CourseID: f.CourseID, PageLimit: f.Size, PageOffset: (f.Page - 1) * f.Size})
	if err != nil {
		return result, err
	}
	ids := make([]int32, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	progress, err := q.LinkedCoursePeopleProgress(ctx, dbgen.LinkedCoursePeopleProgressParams{Kind: scope.Kind, OwnerID: scope.ID, RegistrationIds: ids})
	if err != nil {
		return result, err
	}
	byID := map[int32][]Progress{}
	for _, p := range progress {
		byID[p.RegistrationID] = append(byID[p.RegistrationID], Progress{p.CourseID, p.Status, p.CompletedLessons, p.TotalLessons})
	}
	for _, row := range rows {
		p := byID[row.ID]
		if p == nil {
			p = []Progress{}
		}
		result.Data = append(result.Data, Person{row, p})
	}
	return result, nil
}
func (s Service) People(ctx context.Context, scope Scope, f Filters) (Result, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Result{}, err
	}
	defer tx.Rollback(ctx)
	result, err := read(ctx, dbgen.New(tx), scope, f)
	if err != nil {
		return result, err
	}
	return result, tx.Commit(ctx)
}
func (s Service) Export(ctx context.Context, scope Scope) ([]byte, error) {
	result, err := s.People(ctx, scope, Filters{Page: 1, Size: 2147483647})
	if err != nil {
		return nil, err
	}
	headers := []string{"No", "Nama Lengkap", "Email", "Status Pendaftaran"}
	for _, c := range result.Courses {
		prefix := fmt.Sprintf("%s (#%d)", c.Title, c.ID)
		headers = append(headers, prefix+" - Status", prefix+" - Materi selesai/total")
	}
	data := make([][]interface{}, 0, len(result.Data))
	for i, p := range result.Data {
		row := []interface{}{i + 1, p.Name, p.Email, p.RegistrationStatus}
		for _, progress := range p.CourseProgress {
			count := ""
			if progress.Status != "unverifiable" {
				count = fmt.Sprintf("%d/%d", progress.CompletedLessons, progress.TotalLessons)
			}
			row = append(row, progressLabel(progress.Status), count)
		}
		data = append(data, row)
	}
	return export.Workbook("Progres Kelas", headers, data)
}
func progressLabel(status string) string {
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
