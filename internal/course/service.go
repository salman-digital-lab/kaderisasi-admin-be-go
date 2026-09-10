package course

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"kaderisasi/admin/internal/storage"
)

type Service struct {
	Pool    *pgxpool.Pool
	Storage storage.Store
}
type Meta struct {
	Total       int64 `json:"total"`
	PerPage     int32 `json:"per_page"`
	CurrentPage int32 `json:"current_page"`
	LastPage    int64 `json:"last_page"`
}
type Page[T any] struct {
	Data []T  `json:"data"`
	Meta Meta `json:"meta"`
}
type Document struct {
	ID        int32  `json:"id"`
	LessonID  int32  `json:"lesson_id"`
	Filename  string `json:"filename"`
	SizeBytes int32  `json:"size_bytes"`
}
type Lesson struct {
	dbgen.CourseLesson
	Documents []Document `json:"documents"`
}
type Detail struct {
	dbgen.Course
	Lessons []Lesson `json:"lessons"`
}

func page[T any](rows []T, total int64, number, size int32) Page[T] {
	return Page[T]{Data: rows, Meta: Meta{Total: total, PerPage: size, CurrentPage: number, LastPage: max(1, (total+int64(size)-1)/int64(size))}}
}
func missing(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Fail(404, "COURSE_NOT_FOUND")
	}
	return err
}
func document(row dbgen.CourseDocument) Document {
	return Document{ID: row.ID, LessonID: row.LessonID, Filename: row.Filename, SizeBytes: row.SizeBytes}
}

func (s Service) List(ctx context.Context, search, status string, number, size int32) (Page[dbgen.ListCoursesRow], error) {
	q := dbgen.New(s.Pool)
	rows, err := q.ListCourses(ctx, dbgen.ListCoursesParams{Search: search, Status: status, PageOffset: (number - 1) * size, PageLimit: size})
	if err != nil {
		return Page[dbgen.ListCoursesRow]{}, err
	}
	total, err := q.CountCourses(ctx, dbgen.CountCoursesParams{Search: search, Status: status})
	return page(rows, total, number, size), err
}
func (s Service) Show(ctx context.Context, id int32) (Detail, error) {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Detail{}, err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	row, err := q.CourseByID(ctx, id)
	if err != nil {
		return Detail{}, missing(err)
	}
	lessons, err := q.CourseLessons(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	documents, err := q.CourseDocuments(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	byLesson := map[int32][]Document{}
	for _, d := range documents {
		byLesson[d.LessonID] = append(byLesson[d.LessonID], document(d))
	}
	result := Detail{Course: row, Lessons: []Lesson{}}
	for _, l := range lessons {
		docs := byLesson[l.ID]
		if docs == nil {
			docs = []Document{}
		}
		result.Lessons = append(result.Lessons, Lesson{CourseLesson: l, Documents: docs})
	}
	return result, tx.Commit(ctx)
}
func (s Service) Create(ctx context.Context, in Input) (dbgen.Course, error) {
	if err := in.Validate(true); err != nil {
		return dbgen.Course{}, err
	}
	return dbgen.New(s.Pool).CreateCourse(ctx, dbgen.CreateCourseParams{Title: in.Title, Summary: in.Summary, Description: in.Description, MinimumLevel: in.MinimumLevel})
}
func (s Service) locked(ctx context.Context, id int32, change func(*dbgen.Queries, dbgen.Course) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := dbgen.New(tx)
	current, err := q.LockCourse(ctx, id)
	if err != nil {
		return missing(err)
	}
	if err = change(q, current); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func publishable(lessons []dbgen.CourseLesson) error {
	if len(lessons) == 0 {
		return domain.Fail(422, "COURSE_REQUIRES_LESSON")
	}
	for _, l := range lessons {
		if !videoIDPattern.MatchString(l.YoutubeVideoID) {
			return domain.Fail(422, "COURSE_REQUIRES_VALID_VIDEOS")
		}
	}
	return nil
}
func (s Service) Update(ctx context.Context, id int32, in Input) (dbgen.Course, error) {
	var result dbgen.Course
	if err := in.Validate(false); err != nil {
		return result, err
	}
	err := s.locked(ctx, id, func(q *dbgen.Queries, _ dbgen.Course) error {
		if in.Status == "published" {
			lessons, err := q.CourseLessons(ctx, id)
			if err != nil {
				return err
			}
			if err = publishable(lessons); err != nil {
				return err
			}
		}
		var err error
		result, err = q.UpdateCourse(ctx, dbgen.UpdateCourseParams{ID: id, Title: in.Title, Summary: in.Summary, Description: in.Description, MinimumLevel: in.MinimumLevel, Status: in.Status})
		return err
	})
	return result, err
}
func (s Service) SaveLesson(ctx context.Context, id, lessonID int32, in LessonInput) (dbgen.CourseLesson, error) {
	var result dbgen.CourseLesson
	video, err := in.Validate()
	if err != nil {
		return result, err
	}
	err = s.locked(ctx, id, func(q *dbgen.Queries, c dbgen.Course) error {
		if c.Status == "published" && video == "" {
			return domain.Fail(422, "COURSE_REQUIRES_VALID_VIDEOS")
		}
		var err error
		if lessonID == 0 {
			result, err = q.CreateCourseLesson(ctx, dbgen.CreateCourseLessonParams{CourseID: id, Title: in.Title, Description: in.Description, YoutubeVideoID: video})
		} else {
			result, err = q.UpdateCourseLesson(ctx, dbgen.UpdateCourseLessonParams{CourseID: id, ID: lessonID, Title: in.Title, Description: in.Description, YoutubeVideoID: video})
		}
		return missing(err)
	})
	return result, err
}
func (s Service) RemoveLesson(ctx context.Context, id, lessonID int32) error {
	return s.locked(ctx, id, func(q *dbgen.Queries, c dbgen.Course) error {
		if _, err := q.CourseLessonByID(ctx, dbgen.CourseLessonByIDParams{CourseID: id, ID: lessonID}); err != nil {
			return missing(err)
		}
		if err := q.RemoveCourseLesson(ctx, dbgen.RemoveCourseLessonParams{CourseID: id, ID: lessonID}); err != nil {
			return err
		}
		if c.Status == "published" {
			lessons, err := q.CourseLessons(ctx, id)
			if err != nil {
				return err
			}
			return publishable(lessons)
		}
		return nil
	})
}
func (s Service) Reorder(ctx context.Context, id int32, ids []int32) error {
	return s.locked(ctx, id, func(q *dbgen.Queries, _ dbgen.Course) error {
		lessons, err := q.CourseLessons(ctx, id)
		if err != nil {
			return err
		}
		if len(ids) != len(lessons) {
			return domain.Fail(422, "INVALID_LESSON_ORDER")
		}
		remaining := map[int32]bool{}
		for _, l := range lessons {
			remaining[l.ID] = true
		}
		for i, lessonID := range ids {
			if !remaining[lessonID] {
				return domain.Fail(422, "INVALID_LESSON_ORDER")
			}
			delete(remaining, lessonID)
			if err = q.ReorderCourseLesson(ctx, dbgen.ReorderCourseLessonParams{CourseID: id, ID: lessonID, Position: int32(i + 1)}); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s Service) Learners(ctx context.Context, id int32, search string, number, size int32) (Page[dbgen.CourseLearnersRow], error) {
	q := dbgen.New(s.Pool)
	if _, err := q.CourseByID(ctx, id); err != nil {
		return Page[dbgen.CourseLearnersRow]{}, missing(err)
	}
	rows, err := q.CourseLearners(ctx, dbgen.CourseLearnersParams{CourseID: id, Search: search, PageLimit: size, PageOffset: (number - 1) * size})
	if err != nil {
		return Page[dbgen.CourseLearnersRow]{}, err
	}
	total, err := q.CountCourseLearners(ctx, dbgen.CountCourseLearnersParams{CourseID: id, Search: search})
	return page(rows, total, number, size), err
}
