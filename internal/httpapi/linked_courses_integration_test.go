//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/xuri/excelize/v2"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/linkedcourse"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func testLinkedCoursePeople(t *testing.T, f *activityCourseFixture, activityID int32, courses, users, lessons []int32) {
	t.Helper()
	ctx := context.Background()
	var clubID int32
	uuid, _ := auth.UUID()
	if err := f.pool.QueryRow(ctx, `INSERT INTO clubs(name) VALUES($1) RETURNING id`, uuid).Scan(&clubID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := f.pool.Exec(ctx, "DELETE FROM clubs WHERE id=$1", clubID); err != nil {
			t.Error(err)
		}
	})
	for i, user := range users {
		status := "PENDING"
		if i == 0 {
			status = "APPROVED"
		}
		if _, err := f.pool.Exec(ctx, "INSERT INTO club_registrations(club_id,member_id,status) VALUES($1,$2,$3)", clubID, user, status); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.pool.Exec(ctx, "INSERT INTO club_registrations(club_id,status) VALUES($1,'PENDING')", clubID); err != nil {
		t.Fatal(err)
	}
	manager := f.tokenFor(f.admin("club_manager"))
	denied := f.tokenFor(f.admin("course_manager"))
	base := fmt.Sprintf("/v2/clubs/%d", clubID)
	f.call("GET", "/v2/courses", nil, manager, 200)
	for _, route := range []struct{ method, path string }{{"GET", "/v2/clubs/course-options"}, {"GET", base + "/courses"}, {"PUT", base + "/courses"}, {"GET", base + "/course-progress"}, {"GET", base + "/course-progress/export"}} {
		f.call(route.method, route.path, nil, "", 401)
		f.call(route.method, route.path, map[string]interface{}{"course_ids": []int32{}}, denied, 403)
	}
	f.call("GET", "/v2/clubs/course-options?search="+uuid, nil, manager, 200)
	for _, ids := range [][]int32{nil, {0}, {courses[0], courses[0]}, {2147483647}} {
		f.call("PUT", base+"/courses", linkedcourse.LinksInput{CourseIDs: ids}, manager, 422)
	}
	for _, route := range []struct{ method, suffix string }{{"GET", "/courses"}, {"PUT", "/courses"}, {"GET", "/course-progress"}, {"GET", "/course-progress/export"}} {
		f.call(route.method, "/v2/clubs/2147483647"+route.suffix, linkedcourse.LinksInput{CourseIDs: []int32{}}, manager, 404)
	}
	f.call("GET", base+"/course-progress?course_completion=completed", nil, manager, 422)
	f.call("PUT", base+"/courses", linkedcourse.LinksInput{CourseIDs: []int32{courses[1], courses[0]}}, manager, 200)
	var saved []linkedcourse.Course
	response := f.call("GET", base+"/courses", nil, manager, 200)
	json.Unmarshal(response["data"], &saved)
	if len(saved) != 2 || saved[0].ID != courses[1] || saved[1].ID != courses[0] {
		t.Fatal("club selection order")
	}
	check := func(path, token string, total int64) linkedcourse.Result {
		t.Helper()
		var result linkedcourse.Result
		response := f.call("GET", path, nil, token, 200)
		if err := json.Unmarshal(response["data"], &result); err != nil {
			t.Fatal(err)
		}
		if result.Meta.Total != total {
			t.Fatalf("%s: total %d want %d", path, result.Meta.Total, total)
		}
		return result
	}
	for _, kind := range []string{"club", "activity"} {
		path := base
		token := manager
		if kind == "activity" {
			path = fmt.Sprintf("/v2/activities/%d", activityID)
			token = f.tokenFor(f.admin("activity_manager"))
			f.call("PUT", path+"/courses", linkedcourse.LinksInput{CourseIDs: []int32{courses[1], courses[0]}}, token, 200)
		}
		progress := path + "/course-progress"
		f.call("GET", progress+"/export", nil, "", 401)
		f.call("GET", progress+"/export", nil, denied, 403)
		absent := fmt.Sprintf("/v2/%s/2147483647/course-progress/export", map[string]string{"activity": "activities", "club": "clubs"}[kind])
		f.call("GET", absent, nil, token, 404)

		all := check(progress+"?per_page=50", token, 4)
		if len(all.Data) != 4 || len(all.Courses) != 2 {
			t.Fatal("progress page metadata")
		}
		statuses := map[string]int{}
		for _, p := range all.Data {
			if len(p.CourseProgress) != 2 {
				t.Fatal("missing courses")
			}
			for _, item := range p.CourseProgress {
				statuses[item.Status]++
			}
		}
		if statuses["completed"] != 2 || statuses["in_progress"] != 1 || statuses["not_started"] != 3 || statuses["unverifiable"] != 2 {
			t.Fatalf("states: %+v", statuses)
		}
		check(progress+"?course_completion=completed", token, 1)
		p1 := check(progress+"?course_completion=incomplete&per_page=1", token, 2)
		p2 := check(progress+"?course_completion=incomplete&per_page=1&page=2", token, 2)
		if p1.Data[0].ID == p2.Data[0].ID {
			t.Fatal("duplicated pagination")
		}
		check(progress+"?course_completion=unverifiable", token, 1)
		status := "PENDING"
		if kind == "activity" {
			status = "TERDAFTAR"
		}
		check(progress+"?course_completion=incomplete&search=learner+1&status="+status, token, 1)
		check(progress+fmt.Sprintf("?course_id=%d&course_completion=completed", courses[0]), token, 1)
		for _, query := range []string{"?course_id=0", "?course_id=bad", "?course_id=2147483647", "?course_completion=bad"} {
			f.call("GET", progress+query, nil, token, 422)
		}
		f.call("GET", progress, nil, "", 401)
		f.call("GET", progress, nil, denied, 403)
		f.call("GET", strings.Replace(path, fmt.Sprint(map[string]int32{"activity": activityID, "club": clubID}[kind]), "2147483647", 1)+"/course-progress", nil, token, 404)
		r := httptest.NewRequest("GET", progress+"/export?course_completion=completed", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		f.handler.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("progress export: %s", w.Body)
		}
		f.checks = append(f.checks, activityCourseCheck{Owner: "admin", Label: "export all progress", Method: "GET", Path: strings.TrimPrefix(progress+"/export", "/v2"), Status: 200})
		workbook, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		cells, err := workbook.GetRows("Progres Kelas")
		workbook.Close()
		if err != nil || len(cells) != 5 || !strings.Contains(cells[0][4], fmt.Sprintf("(#%d)", courses[1])) {
			t.Fatal("progress export layout/all registrants", err)
		}
		completed := 0
		for _, row := range cells[1:] {
			if len(row) > 5 && row[4] == "Selesai" && row[5] == "1/1" {
				completed++
			}
		}
		if completed != 1 {
			t.Fatal("export status mismatch")
		}
	}
	progress := base + "/course-progress"
	exec := func(query string, args ...interface{}) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec("UPDATE course_lessons SET deleted_at=now() WHERE id=$1", lessons[0])
	check(progress+"?course_completion=completed", manager, 0)
	check(progress+"?course_completion=incomplete", manager, 3)
	exec("UPDATE course_lessons SET deleted_at=NULL WHERE id=$1", lessons[0])
	exec("UPDATE course_lesson_progress SET completed_at=NULL WHERE user_id=$1 AND lesson_id=$2", users[0], lessons[0])
	check(progress+"?course_completion=completed", manager, 0)
	exec("UPDATE course_lesson_progress SET completed_at=now() WHERE user_id=$1 AND lesson_id=$2", users[0], lessons[0])
	check(progress+"?course_completion=completed", manager, 1)
	f.call("PUT", base+"/courses", linkedcourse.LinksInput{CourseIDs: courses}, manager, 200)
	empty := check(progress, manager, 4)
	for _, p := range empty.Data {
		if p.CourseProgress[0].Status != "unverifiable" && p.CourseProgress[2].Status != "empty" {
			t.Fatal("empty course")
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, ids := range [][]int32{{courses[0]}, {courses[1], courses[2]}} {
		wg.Go(func() {
			_, err := (linkedcourse.Service{Pool: f.pool}).SaveClubLinks(ctx, clubID, linkedcourse.LinksInput{CourseIDs: ids})
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	response = f.call("GET", base+"/courses", nil, manager, 200)
	json.Unmarshal(response["data"], &saved)
	if !(len(saved) == 1 && saved[0].ID == courses[0]) && !(len(saved) == 2 && saved[0].ID == courses[1] && saved[1].ID == courses[2]) {
		t.Fatal("interleaved club links")
	}
	f.call("PUT", base+"/courses", linkedcourse.LinksInput{CourseIDs: []int32{}}, manager, 200)
	cleared := check(progress, manager, 4)
	if len(cleared.Courses) != 0 || len(cleared.Data[0].CourseProgress) != 0 {
		t.Fatal("club clear")
	}
	f.call("GET", progress+"?course_completion=completed", nil, manager, 422)
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM course_lesson_progress WHERE user_id=ANY($1::int[])", users).Scan(&count); err != nil || count != 3 {
		t.Fatal("club unlink changed learning history")
	}
}
