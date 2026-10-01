//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/talent"
)

type talentCheck struct {
	Owner  string `json:"owner"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Label  string `json:"label"`
	Status int    `json:"status"`
}
type talentFixture struct {
	*httpFixture
	checks []talentCheck
}

func (f *talentFixture) call(method, path string, body interface{}, token string, status int, label string) database.Object {
	f.t.Helper()
	result := f.httpFixture.call(method, path, body, token, status)
	f.checks = append(f.checks, talentCheck{"admin", method, path[3:], label, status})
	return result
}
func decodeTalent[T any](t *testing.T, response database.Object) T {
	t.Helper()
	var value T
	if err := json.Unmarshal(response["data"], &value); err != nil {
		t.Fatal(err)
	}
	return value
}
func TestTalentAssessmentWorkflow(t *testing.T) {
	f := &talentFixture{httpFixture: newHTTPFixture(t)}
	participant := f.admin("")
	token := f.tokenFor(participant)
	other := f.admin("admin")
	otherToken := f.tokenFor(other)
	base := "/v2/talent-assessment"
	adminPath := fmt.Sprintf("/v2/admin-users/%d/talent-assessment/result", participant)
	for _, route := range []struct{ method, path string }{{"GET", base}, {"GET", base + "/definition"}, {"POST", base + "/draft"}, {"PUT", base + "/draft"}, {"POST", base + "/submit"}, {"GET", base + "/result"}, {"GET", adminPath}} {
		f.call(route.method, route.path, nil, "", 401, "authentication")
	}
	state := decodeTalent[talent.State](t, f.call("GET", base, nil, token, 200, "empty state"))
	if state.Draft != nil || state.Result != nil {
		t.Fatal(state)
	}
	definition := decodeTalent[talent.ParticipantDefinition](t, f.call("GET", base+"/definition", nil, token, 200, "participant definition"))
	if len(definition.Questions) != 170 {
		t.Fatal("missing questions")
	}
	f.call("GET", base+"/result", nil, token, 404, "missing result")
	f.call("GET", adminPath, nil, otherToken, 403, "permission denied")
	f.call("GET", "/v2/admin-users/bad/talent-assessment/result", nil, f.token, 404, "invalid identifier")
	f.call("GET", "/v2/admin-users/2147483647/talent-assessment/result", nil, f.token, 404, "missing account result")
	draft := decodeTalent[talent.Draft](t, f.call("POST", base+"/draft", map[string]int32{"admin_user_id": other}, token, 200, "start self-owned draft"))
	resumed := decodeTalent[talent.Draft](t, f.call("POST", base+"/draft", nil, token, 200, "resume existing draft"))
	if resumed.ID != draft.ID {
		t.Fatal("start reset draft")
	}
	f.call("PUT", base+"/draft", nil, token, 422, "invalid missing request")
	f.call("POST", base+"/submit", nil, token, 422, "invalid missing submission")
	for _, answers := range [][]int{{1}, make([]int, 170)} {
		bad := talent.SaveRequest{DraftID: draft.ID, Revision: draft.Revision, Answers: answers, CurrentQuestion: 1}
		if len(answers) == 170 {
			bad.Answers[0] = 7
		}
		f.call("PUT", base+"/draft", bad, token, 422, "invalid answers")
	}
	f.call("POST", base+"/submit", talent.SubmitRequest{DraftID: draft.ID, Revision: draft.Revision}, token, 422, "invalid incomplete submission")
	save := talent.SaveRequest{DraftID: draft.ID, Revision: draft.Revision, Answers: draft.Answers, CurrentQuestion: 2}
	save.Answers[0] = 6
	draft = decodeTalent[talent.Draft](t, f.call("PUT", base+"/draft", save, token, 200, "save and resume progress"))
	f.call("PUT", base+"/draft", save, token, 409, "invalid stale revision")
	f.call("PUT", base+"/draft", talent.SaveRequest{DraftID: draft.ID, Revision: draft.Revision, Answers: draft.Answers, CurrentQuestion: 2}, otherToken, 409, "wrong owner draft")
	otherState := decodeTalent[talent.State](t, f.call("GET", base, nil, otherToken, 200, "other account isolation"))
	if otherState.Draft != nil {
		t.Fatal("draft leaked")
	}
	// Two tabs saving the same revision must produce one success and one conflict.
	body, _ := json.Marshal(talent.SaveRequest{DraftID: draft.ID, Revision: draft.Revision, Answers: draft.Answers, CurrentQuestion: 3})
	statuses := make(chan int, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			r := httptest.NewRequest("PUT", base+"/draft", bytes.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			f.handler.ServeHTTP(w, r)
			statuses <- w.Code
		})
	}
	wg.Wait()
	close(statuses)
	successes, conflicts := 0, 0
	for status := range statuses {
		if status == 200 {
			successes++
		} else if status == 409 {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatal("concurrent writes", successes, conflicts)
	}
	state = decodeTalent[talent.State](t, f.call("GET", base, nil, token, 200, "resume after concurrent write"))
	draft = *state.Draft
	answers := make([]int, 170)
	for i := range answers {
		answers[i] = 6
	}
	draft = decodeTalent[talent.Draft](t, f.call("PUT", base+"/draft", talent.SaveRequest{DraftID: draft.ID, Revision: draft.Revision, Answers: answers, CurrentQuestion: 170}, token, 200, "save complete answers"))
	submission := talent.SubmitRequest{DraftID: draft.ID, Revision: draft.Revision}
	result := decodeTalent[talent.Result](t, f.call("POST", base+"/submit", submission, token, 200, "submit server-scored result"))
	retry := decodeTalent[talent.Result](t, f.call("POST", base+"/submit", submission, token, 200, "duplicate submission retry"))
	if retry.SubmittedAt != result.SubmittedAt {
		t.Fatal("duplicate replaced result")
	}
	if len(result.Talents) != 34 || result.Talents[0].Score != 100 {
		t.Fatal(result)
	}
	f.call("GET", base+"/result", nil, token, 200, "self result")
	superResult := f.call("GET", adminPath, nil, f.token, 200, "super admin completed result")
	if bytes.Contains(superResult["data"], []byte(`"answers"`)) {
		t.Fatal("raw answers exposed")
	}
	// Additional super-admin role uses the same authorization semantics.
	if _, err := f.pool.Exec(context.Background(), "UPDATE admin_users SET additional_role_codes=ARRAY['super_admin'] WHERE id=$1", other); err != nil {
		t.Fatal(err)
	}
	f.call("GET", adminPath, nil, otherToken, 200, "additional super admin role")
	retake := decodeTalent[talent.Draft](t, f.call("POST", base+"/draft", nil, token, 200, "start retake"))
	state = decodeTalent[talent.State](t, f.call("GET", base, nil, token, 200, "retake preserves result"))
	if state.Result.SubmissionID != result.SubmissionID || retake.ID == result.SubmissionID {
		t.Fatal("retake destroyed previous result")
	}
	f.call("POST", base+"/submit", submission, token, 200, "retry old submission while retaking")
	for i := range answers {
		answers[i] = 1
	}
	retake = decodeTalent[talent.Draft](t, f.call("PUT", base+"/draft", talent.SaveRequest{DraftID: retake.ID, Revision: retake.Revision, Answers: answers, CurrentQuestion: 170}, token, 200, "retake complete answers"))
	f.call("POST", base+"/submit", talent.SubmitRequest{DraftID: retake.ID, Revision: retake.Revision}, token, 200, "replace previous result")
	state = decodeTalent[talent.State](t, f.call("GET", base, nil, token, 200, "replacement has no draft or history"))
	if state.Draft != nil || state.Result.SubmissionID != retake.ID || state.Result.Talents[0].Score != 0 {
		t.Fatal(state)
	}
	var rows int
	if err := f.pool.QueryRow(context.Background(), "SELECT count(*) FROM talent_assessment_results WHERE admin_user_id=$1", participant).Scan(&rows); err != nil || rows != 1 {
		t.Fatal("result history persisted", err, rows)
	}
	f.call("PUT", base+"/draft", save, token, 409, "missing draft after submission")
	f.call("POST", base+"/submit", talent.SubmitRequest{DraftID: "missing-draft", Revision: 1}, token, 409, "missing submission draft")
	encoded, _ := json.Marshal(f.checks)
	if err := os.WriteFile(filepath.Join(os.Getenv("GO_REWRITE_ARTIFACTS"), "talent-assessment-cases.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
}
