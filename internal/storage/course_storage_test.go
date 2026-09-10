package storage

import (
	"context"
	"io"
	"kaderisasi/admin/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCourseDocumentsUsePrivateUploads(t *testing.T) {
	var acl, cache, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acl = r.Header.Get("X-Amz-Acl")
		cache = r.Header.Get("Cache-Control")
		path = r.URL.Path
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(200)
	}))
	defer server.Close()
	c := config.Config{DriveRegion: "test", DriveKey: "synthetic", DriveSecret: "synthetic", DriveEndpoint: server.URL, DriveDisk: "minio", DriveBucket: "shared"}
	store := NewCourseDocuments(c)
	if err := store.Put(context.Background(), "courses/lesson.pdf", []byte("%PDF-1.7"), "application/pdf"); err != nil {
		t.Fatal(err)
	}
	if acl != "private" || cache != "private, no-store" || path != "/shared/courses/lesson.pdf" {
		t.Fatalf("private upload headers: %s %s %s", acl, cache, path)
	}
	if err := New(c).Put(context.Background(), "logo.png", []byte("fixture"), "image/png"); err != nil {
		t.Fatal(err)
	}
	if acl != "public-read" || cache != "public, max-age=31536000, immutable" {
		t.Fatal("public upload behavior changed")
	}
	c.DriveBucket = ""
	if NewCourseDocuments(c) != nil {
		t.Fatal("missing bucket configuration must fail closed")
	}
}
