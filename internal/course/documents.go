package course

import (
	"bytes"
	"context"
	"errors"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"kaderisasi/admin/internal/domain"
	"path/filepath"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func ValidatePDF(filename string, body []byte) (string, error) {
	filename = filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	filename = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, filename)
	if len(body) == 0 || len(body) > MaxPDFBytes || utf8.RuneCountInString(filename) > 255 || strings.ToLower(filepath.Ext(filename)) != ".pdf" || !bytes.HasPrefix(body, []byte("%PDF-")) || !bytes.Contains(body[max(0, len(body)-1024):], []byte("%%EOF")) {
		return "", domain.Fail(422, "INVALID_PDF")
	}
	return filename, nil
}
func (s Service) Upload(ctx context.Context, id, lessonID int32, filename string, body []byte) (Document, error) {
	var result Document
	filename, err := ValidatePDF(filename, body)
	if err != nil {
		return result, err
	}
	if s.Storage == nil {
		return result, domain.Fail(503, "COURSE_STORAGE_UNAVAILABLE")
	}
	err = s.locked(ctx, id, func(q *dbgen.Queries, _ dbgen.Course) error {
		if _, err := q.CourseLessonByID(ctx, dbgen.CourseLessonByIDParams{CourseID: id, ID: lessonID}); err != nil {
			return missing(err)
		}
		objectID, err := auth.UUID()
		if err != nil {
			return err
		}
		key := "courses/" + objectID + ".pdf"
		if err := s.Storage.Put(ctx, key, body, "application/pdf"); err != nil {
			return err
		}
		row, err := q.CreateCourseDocument(ctx, dbgen.CreateCourseDocumentParams{LessonID: lessonID, StorageKey: key, Filename: filename, SizeBytes: int32(len(body))})
		if err != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
			defer cancel()
			return errors.Join(err, s.Storage.Delete(cleanup, key))
		}
		result = document(row)
		return nil
	})
	return result, err
}
func (s Service) Download(ctx context.Context, id, lessonID, documentID int32) (Document, []byte, error) {
	row, err := dbgen.New(s.Pool).CourseDocumentByID(ctx, dbgen.CourseDocumentByIDParams{CourseID: id, ID: lessonID, ID_2: documentID})
	if err != nil {
		return Document{}, nil, missing(err)
	}
	if s.Storage == nil {
		return Document{}, nil, domain.Fail(503, "COURSE_STORAGE_UNAVAILABLE")
	}
	body, err := s.Storage.Get(ctx, row.StorageKey)
	return document(row), body, err
}
func (s Service) RemoveDocument(ctx context.Context, id, lessonID, documentID int32) error {
	return s.locked(ctx, id, func(q *dbgen.Queries, _ dbgen.Course) error {
		row, err := q.CourseDocumentByID(ctx, dbgen.CourseDocumentByIDParams{CourseID: id, ID: lessonID, ID_2: documentID})
		if err != nil {
			return missing(err)
		}
		// Retain the private object with the archived lesson history.
		return q.RemoveCourseDocument(ctx, row.ID)
	})
}
