//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/xuri/excelize/v2"
	"kaderisasi/admin/internal/activity"
	"kaderisasi/admin/internal/auth"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

type activityCourseCheck struct {
	Owner  string `json:"owner"`
	Label  string `json:"label"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
}
type activityCourseFixture struct {
	*httpFixture
	checks []activityCourseCheck
}

func (f *activityCourseFixture) call(method, path string, body interface{}, token string, status int) database.Object {
	f.t.Helper()
	result := f.httpFixture.call(method, path, body, token, status)
	label := method + " " + path
	if status == 422 || strings.Contains(path, "/bad/") {
		label = "invalid-input: " + label
	}
	if strings.Contains(path, "2147483647/courses") {
		label = "missing-resource: " + label
	}
	f.checks = append(f.checks, activityCourseCheck{Owner: "admin", Label: label, Method: method, Path: strings.TrimPrefix(path, "/v2"), Status: status})
	return result
}

func TestActivityCoursesWorkflow(t *testing.T) {
	f := &activityCourseFixture{httpFixture: newHTTPFixture(t)}
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		raw, err := json.Marshal(f.checks)
		if err != nil {
			t.Error(err)
			return
		}
		if err = os.WriteFile(filepath.Join(os.Getenv("GO_REWRITE_ARTIFACTS"), "activity-courses-cases.json"), raw, 0600); err != nil {
			t.Error(err)
		}
	})
	ctx := context.Background()
	uuid, _ := auth.UUID()
	var activityID int32
	if err := f.pool.QueryRow(ctx, `INSERT INTO activities(name,slug,additional_config,created_at,updated_at) VALUES('Linked course fixture',$1,'{"mandatory_profile_data":[],"additional_questionnaire":[]}',now(),now()) RETURNING id`, uuid).Scan(&activityID); err != nil {
		t.Fatal(err)
	}
	courses := []int32{}
	users := []int32{}
	t.Cleanup(func() {
		for _, target := range []struct {
			table string
			ids   []int32
		}{{"activities", []int32{activityID}}, {"courses", courses}, {"public_users", users}} {
			if _, err := f.pool.Exec(ctx, "DELETE FROM "+target.table+" WHERE id=ANY($1::int[])", target.ids); err != nil {
				t.Error(err)
			}
		}
	})
	for _, status := range []string{"draft", "published", "archived"} {
		var id int32
		if err := f.pool.QueryRow(ctx, `INSERT INTO courses(title,status,minimum_level) VALUES($1,$2,0) RETURNING id`, "Same course title "+uuid, status).Scan(&id); err != nil {
			t.Fatal(err)
		}
		courses = append(courses, id)
	}
	lessons := []int32{}
	for _, courseID := range courses[:2] {
		var lessonID int32
		if err := f.pool.QueryRow(ctx, `INSERT INTO course_lessons(course_id,title,youtube_video_id,position) VALUES($1,'Lesson','dQw4w9WgXcQ',1) RETURNING id`, courseID).Scan(&lessonID); err != nil {
			t.Fatal(err)
		}
		lessons = append(lessons, lessonID)
	}
	regs := []int32{}
	for i := 0; i < 3; i++ {
		var userID, regID int32
		if err := f.pool.QueryRow(ctx, `INSERT INTO public_users(email,password,created_at,updated_at) VALUES($1,'fixture',now(),now()) RETURNING id`, fmt.Sprintf("%s-%d@example.test", uuid, i)).Scan(&userID); err != nil {
			t.Fatal(err)
		}
		users = append(users, userID)
		if _, err := f.pool.Exec(ctx, `INSERT INTO profiles(user_id,name,created_at,updated_at) VALUES($1,$2,now(),now())`, userID, fmt.Sprintf("Course learner %d", i)); err != nil {
			t.Fatal(err)
		}
		if err := f.pool.QueryRow(ctx, `INSERT INTO activity_registrations(activity_id,user_id,status,created_at,updated_at) VALUES($1,$2,'TERDAFTAR',now(),now()) RETURNING id`, activityID, userID).Scan(&regID); err != nil {
			t.Fatal(err)
		}
		regs = append(regs, regID)
	}
	var guestID int32
	if err := f.pool.QueryRow(ctx, `INSERT INTO activity_registrations(activity_id,guest_data,status,created_at,updated_at) VALUES($1,'{"name":"Guest","email":"guest@example.test"}','TERDAFTAR',now(),now()) RETURNING id`, activityID).Scan(&guestID); err != nil {
		t.Fatal(err)
	}
	exec := func(sql string, args ...interface{}) {
		t.Helper()
		if _, err := f.pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO course_lesson_progress(user_id,lesson_id,completed_at) VALUES($1,$2,now()),($1,$3,now()),($4,$2,NULL)`, users[0], lessons[0], lessons[1], users[1])
	manager := f.tokenFor(f.admin("activity_manager"))
	denied := f.tokenFor(f.admin("course_manager"))
	path := fmt.Sprintf("/v2/activities/%d/courses", activityID)
	listPath := fmt.Sprintf("/v2/activities/%d/registrations", activityID)
	links := func() []dbgen.LinkedActivityCoursesRow {
		t.Helper()
		var rows []dbgen.LinkedActivityCoursesRow
		response := f.call("GET", path, nil, manager, 200)
		if err := json.Unmarshal(response["data"], &rows); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	if len(links()) != 0 {
		t.Fatal("new activity has course links")
	}
	f.call("GET", "/v2/courses", nil, manager, 403)
	options := objectData(t, f.call("GET", "/v2/activities/course-options?search="+uuid+"&per_page=2", nil, manager, 200))
	var optionRows []dbgen.ActivityCourseOptionsRow
	json.Unmarshal(options["data"], &optionRows)
	if len(optionRows) != 2 {
		t.Fatal("course option pagination")
	}
	for _, route := range []struct{ method, path string }{{"GET", path}, {"PUT", path}, {"GET", "/v2/activities/course-options"}} {
		f.call(route.method, route.path, nil, "", 401)
		f.call(route.method, route.path, activity.CourseLinksInput{CourseIDs: []int32{}}, denied, 403)
	}
	for _, ids := range [][]int32{nil, {0}, {courses[0], courses[0]}, {2147483647}} {
		f.call("PUT", path, activity.CourseLinksInput{CourseIDs: ids}, manager, 422)
	}
	for _, method := range []string{"GET", "PUT"} {
		f.call(method, "/v2/activities/2147483647/courses", activity.CourseLinksInput{CourseIDs: []int32{}}, manager, 404)
		f.call(method, "/v2/activities/bad/courses", activity.CourseLinksInput{CourseIDs: []int32{}}, manager, 404)
	}
	setLinks := func(ids []int32) {
		t.Helper()
		f.call("PUT", path, activity.CourseLinksInput{CourseIDs: ids}, manager, 200)
	}
	setLinks([]int32{courses[1], courses[0]})
	if got := links(); got[0].ID != courses[1] || got[1].ID != courses[0] {
		t.Fatal("selection order lost")
	}
	page := func(query string, want int) []activity.RegistrationSummary {
		t.Helper()
		data := objectData(t, f.call("GET", listPath+query, nil, manager, 200))
		var rows []activity.RegistrationSummary
		var meta struct {
			Total int `json:"total"`
		}
		if err := json.Unmarshal(data["data"], &rows); err != nil {
			t.Fatal(err)
		}
		json.Unmarshal(data["meta"], &meta)
		if meta.Total != want {
			t.Fatalf("%s total %d want %d", query, meta.Total, want)
		}
		return rows
	}
	all := page("?per_page=50", 4)
	wantStatuses := map[int32][]string{regs[0]: {"completed", "completed"}, regs[1]: {"not_started", "in_progress"}, regs[2]: {"not_started", "not_started"}, guestID: {"unverifiable", "unverifiable"}}
	for _, row := range all {
		if len(row.CourseProgress) != 2 {
			t.Fatal("missing progress")
		}
		for i, p := range row.CourseProgress {
			if p.Status != wantStatuses[row.ID][i] {
				t.Fatalf("registration %d: %+v", row.ID, row.CourseProgress)
			}
		}
	}
	if got := page("?course_completion=completed", 1); got[0].ID != regs[0] {
		t.Fatal("wrong completed registrant")
	}
	if got := page("?course_completion=incomplete&per_page=1&page=2", 2); len(got) != 1 {
		t.Fatal("filtered pagination")
	}
	page("?course_completion=unverifiable", 1)
	page("?course_completion=incomplete&search=learner+1&status=TERDAFTAR", 1)
	page(fmt.Sprintf("?course_id=%d&course_completion=completed", courses[0]), 1)
	for _, query := range []string{"?course_id=0", "?course_id=bad", "?course_id=2147483647", "?course_completion=bad"} {
		f.call("GET", listPath+query, nil, manager, 422)
	}
	exec(`UPDATE course_lessons SET deleted_at=now() WHERE id=$1`, lessons[0])
	page("?course_completion=completed", 0)
	page("?course_completion=incomplete", 3)
	exec(`UPDATE course_lessons SET deleted_at=NULL WHERE id=$1`, lessons[0])
	exec(`UPDATE course_lesson_progress SET completed_at=NULL WHERE user_id=$1 AND lesson_id=$2`, users[0], lessons[0])
	page("?course_completion=completed", 0)
	exec(`UPDATE course_lesson_progress SET completed_at=now() WHERE user_id=$1 AND lesson_id=$2`, users[0], lessons[0])
	exec(`UPDATE courses SET status='archived' WHERE id=$1`, courses[0])
	page("?course_completion=completed", 1)
	var added int32
	if err := f.pool.QueryRow(ctx, `INSERT INTO course_lessons(course_id,title,position) VALUES($1,'Added later',2) RETURNING id`, courses[0]).Scan(&added); err != nil {
		t.Fatal(err)
	}
	page("?course_completion=completed", 0)
	exec(`UPDATE course_lessons SET deleted_at=now() WHERE id=$1`, added)
	page("?course_completion=completed", 1)
	setLinks(courses)
	for _, row := range page("", 4) {
		if row.ID != guestID && row.CourseProgress[2].Status != "empty" {
			t.Fatal("empty course marked complete")
		}
	}
	setLinks([]int32{courses[1], courses[0]})
	r := httptest.NewRequest("GET", fmt.Sprintf("/v2/activities/%d/registrations-export", activityID), nil)
	r.Header.Set("Authorization", "Bearer "+manager)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("export: %s", w.Body)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer workbook.Close()
	cells, err := workbook.GetRows("Registrations")
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 5 {
		t.Fatal("export omitted registrants")
	}
	headers := cells[0]
	if headers[len(headers)-4] != fmt.Sprintf("Same course title %s (#%d) - Status", uuid, courses[1]) {
		t.Fatal("export course order/identity")
	}
	completedRows := 0
	for _, row := range cells[1:] {
		if slices.Contains(row, "Selesai") {
			completedRows++
		}
		if slices.Contains(row, "Selesai") && !slices.Contains(row, "1/1") {
			t.Fatal("export counts")
		}
	}
	if completedRows != 1 {
		t.Fatal("export completion differs from list")
	}
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for _, ids := range [][]int32{{courses[0]}, {courses[1], courses[2]}} {
		wg.Go(func() {
			_, err := (activity.Service{Pool: f.pool}).SaveLinkedCourses(ctx, activityID, activity.CourseLinksInput{CourseIDs: ids})
			errors <- err
		})
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := links()
	if !(len(got) == 1 && got[0].ID == courses[0]) && !(len(got) == 2 && got[0].ID == courses[1] && got[1].ID == courses[2]) {
		t.Fatal("concurrent replacements interleaved")
	}
	setLinks([]int32{})
	for _, row := range page("", 4) {
		if row.CourseProgress == nil || len(row.CourseProgress) != 0 {
			t.Fatal("unlinked progress is not []")
		}
	}
	f.call("GET", listPath+"?course_completion=completed", nil, manager, 422)
	var count int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM course_lesson_progress WHERE user_id=ANY($1::int[])", users).Scan(&count); err != nil || count != 3 {
		t.Fatal("unlink changed learning history", err, count)
	}
}
