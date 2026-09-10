//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestCustomFormAttachmentConcurrency(t *testing.T) {
	f := newHTTPFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	var clubs, forms []int32
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		for _, target := range []struct {
			table string
			ids   []int32
		}{{"custom_forms", forms}, {"clubs", clubs}} {
			if _, err := f.pool.Exec(cleanup, "DELETE FROM "+target.table+" WHERE id=ANY($1::int[])", target.ids); err != nil {
				t.Error(err)
			}
		}
	})
	for i := 0; i < 2; i++ {
		club := objectData(t, f.call("POST", "/v2/clubs", map[string]string{"name": fmt.Sprintf("Form locking club %d", i)}, f.token, 200))
		clubs = append(clubs, club.ID("id"))
		form := objectData(t, f.call("POST", "/v2/custom-forms", map[string]string{"formName": fmt.Sprintf("Form locking %d", i)}, f.token, 201))
		forms = append(forms, form.ID("id"))
	}
	type operation struct {
		path  string
		input interface{}
	}
	race := func(ops []operation) map[int]int {
		t.Helper()
		codes := make(chan int, len(ops))
		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, op := range ops {
			wg.Go(func() {
				body, _ := json.Marshal(op.input)
				request := httptest.NewRequest("PUT", op.path, bytes.NewReader(body)).WithContext(ctx)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Authorization", "Bearer "+f.token)
				response := httptest.NewRecorder()
				<-start
				f.handler.ServeHTTP(response, request)
				codes <- response.Code
			})
		}
		close(start)
		wg.Wait()
		close(codes)
		counts := map[int]int{}
		for code := range codes {
			counts[code]++
		}
		return counts
	}
	operations := []operation{}
	for _, id := range forms {
		operations = append(operations, operation{fmt.Sprintf("/v2/custom-forms/%d/attach-club", id), map[string]int32{"clubId": clubs[0]}})
	}
	counts := race(operations)
	if counts[200] != 1 || counts[409] != 1 {
		t.Fatalf("exclusive attachment: %v", counts)
	}
	var attached int
	if err := f.pool.QueryRow(ctx, "SELECT count(*) FROM custom_forms WHERE feature_type='club_registration' AND feature_id=$1", clubs[0]).Scan(&attached); err != nil || attached != 1 {
		t.Fatal("duplicate attachment", attached, err)
	}
	// Opposite moves lock both clubs in the same numeric order. Both fail the
	// attachment invariant and must leave the original links intact.
	for i, id := range forms {
		if _, err := f.pool.Exec(ctx, "UPDATE custom_forms SET feature_type='club_registration',feature_id=$1 WHERE id=$2", clubs[i], id); err != nil {
			t.Fatal(err)
		}
	}
	operations = nil
	for i, id := range forms {
		operations = append(operations, operation{fmt.Sprintf("/v2/custom-forms/%d", id), map[string]int32{"featureId": clubs[1-i]}})
	}
	counts = race(operations)
	if counts[409] != 2 {
		t.Fatalf("opposite moves did not roll back: %v", counts)
	}
	for i, id := range forms {
		var attachedID int32
		if err := f.pool.QueryRow(ctx, "SELECT feature_id FROM custom_forms WHERE id=$1", id).Scan(&attachedID); err != nil || attachedID != clubs[i] {
			t.Fatal("attachment changed after conflict", attachedID, err)
		}
	}
	if _, err := f.pool.Exec(ctx, "UPDATE clubs SET is_registration_open=true WHERE id=$1", clubs[0]); err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/v2/custom-forms/%d", forms[0])
	for _, op := range []operation{{path, map[string]bool{"isActive": false}}, {path + "/detach-club", map[string]string{}}, {path, map[string]interface{}{"formSchema": map[string]interface{}{"fields": []interface{}{}}}}} {
		f.call("PUT", op.path, op.input, f.token, 400)
	}
	f.call("PUT", path, map[string]string{"formName": "An open form may be renamed"}, f.token, 200)
}
