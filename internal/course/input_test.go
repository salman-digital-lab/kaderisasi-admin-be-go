package course

import (
	"bytes"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/dbgen"
	"testing"
)

func TestYouTubeURLs(t *testing.T) {
	for _, value := range []string{"dQw4w9WgXcQ", "https://youtu.be/dQw4w9WgXcQ?t=3", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=ignored", "https://m.youtube.com/shorts/dQw4w9WgXcQ", "https://youtube.com/embed/dQw4w9WgXcQ"} {
		id, err := YouTubeID(value)
		if err != nil || id != "dQw4w9WgXcQ" {
			t.Fatalf("valid URL %q rejected: %s %v", value, id, err)
		}
	}
	for _, value := range []string{"https://youtube.com.evil.test/watch?v=dQw4w9WgXcQ", "javascript:alert(1)", "https://evil.test/youtube.com/watch?v=dQw4w9WgXcQ", "https://user@youtube.com/watch?v=dQw4w9WgXcQ", "https://youtube.com:9000/watch?v=dQw4w9WgXcQ", "https://www.youtube.com/playlist?list=abc", "https://youtu.be/short", "https://youtu.be/dQw4w9WgXcQ/extra"} {
		if _, err := YouTubeID(value); err == nil {
			t.Errorf("accepted unsafe URL %q", value)
		}
	}
}
func TestPublicationAndMinimumLevel(t *testing.T) {
	for _, level := range []int32{0, 3, 6, 10} {
		in := Input{Title: "Course", MinimumLevel: level}
		if err := in.Validate(true); err != nil {
			t.Fatal(err)
		}
		if in.Status != "draft" {
			t.Fatal("new course must be a draft")
		}
	}
	in := Input{Title: "Course", MinimumLevel: 4}
	if in.Validate(true) == nil {
		t.Fatal("accepted unknown level")
	}
	if publishable(nil) == nil || publishable([]dbgen.CourseLesson{{YoutubeVideoID: ""}}) == nil {
		t.Fatal("published incomplete course")
	}
	if err := publishable([]dbgen.CourseLesson{{YoutubeVideoID: "dQw4w9WgXcQ"}}); err != nil {
		t.Fatal(err)
	}
}
func TestPDFBoundaries(t *testing.T) {
	pdf := []byte("%PDF-1.7\nfixture\n%%EOF\n")
	if name, err := ValidatePDF("../../lesson.pdf", pdf); err != nil || name != "lesson.pdf" {
		t.Fatalf("filename normalization: %q %v", name, err)
	}
	maximum := append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte(" "), MaxPDFBytes-len("%PDF-1.7\n")-len("%%EOF"))...)
	maximum = append(maximum, []byte("%%EOF")...)
	if _, err := ValidatePDF("max.pdf", maximum); err != nil {
		t.Fatal("rejected exactly 20 MiB")
	}
	for _, input := range []struct {
		name string
		data []byte
	}{{"text.pdf", []byte("not PDF")}, {"script.html", pdf}, {"empty.pdf", nil}, {"truncated.pdf", []byte("%PDF-1.7\n")}, {"large.pdf", append(maximum, ' ')}} {
		if _, err := ValidatePDF(input.name, input.data); err == nil {
			t.Errorf("accepted invalid PDF %s", input.name)
		}
	}
}
func TestCoursePermissions(t *testing.T) {
	for _, code := range []string{"super_admin", "admin", "club_manager"} {
		access := auth.ForRole(&code, true)
		if !access.Allows("courses.read") || !access.Allows("courses.manage") {
			t.Errorf("%s lacks course access", code)
		}
		if auth.ForRole(&code, false).Allows("courses.manage") {
			t.Error("inactive administrator can manage courses")
		}
	}
	for _, code := range []string{"asmen", "kapro", "activity_manager", "course_manager", "konselor"} {
		if auth.ForRole(&code, true).Allows("courses.manage") {
			t.Errorf("unexpected access for %s", code)
		}
	}
	role := auth.RoleByCode("club_manager")
	if role == nil || !role.IsRequestable {
		t.Fatal("Pengelola Komunitas must be requestable")
	}
}
