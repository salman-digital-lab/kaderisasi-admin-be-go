//go:build integration

package httpapi

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestRegistrationExportAfterChoiceRename(t *testing.T) {
	f := newHTTPFixture(t)
	ctx := context.Background()
	a := objectData(t, f.call("POST", "/v2/activities", map[string]string{"name": "Choice rename fixture"}, f.token, 200))
	id := a.ID("id")
	var formID int32
	defer func() {
		for _, target := range []struct {
			table string
			id    int32
		}{{"custom_forms", formID}, {"activities", id}} {
			if _, err := f.pool.Exec(ctx, "DELETE FROM "+target.table+" WHERE id=$1", target.id); err != nil {
				t.Error(err)
			}
		}
	}()
	schema := `{"fields":[{"section_name":"Answers","fields":[{"key":"choice","label":"Choice","type":"select","options":[{"value":"Old label","label":"Old label"}]}]}]}`
	if err := f.pool.QueryRow(ctx, `INSERT INTO custom_forms(form_name,feature_type,feature_id,form_schema,is_active,created_at,updated_at) VALUES('Choice fixture','activity_registration',$1,$2,true,now(),now()) RETURNING id`, id, schema).Scan(&formID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO activity_registrations(activity_id,guest_data,questionnaire_answer,status,created_at,updated_at) VALUES($1,'{"name":"Choice guest"}','{"choice":"Old label"}','TERDAFTAR',now(),now())`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `UPDATE custom_forms SET form_schema=jsonb_set(form_schema,'{fields,0,fields,0,options,0,label}','"New label"'),updated_at=now() WHERE id=$1`, formID); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", fmt.Sprintf("/v2/activities/%d/registrations-export", id), nil)
	request.Header.Set("Authorization", "Bearer "+f.token)
	response := httptest.NewRecorder()
	f.handler.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("export %d: %s", response.Code, response.Body)
	}
	book, err := excelize.OpenReader(bytes.NewReader(response.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer book.Close()
	rows, err := book.GetRows("Registrations")
	if err != nil || len(rows) != 2 || len(rows[1]) != 31 || rows[1][30] != "New label" {
		t.Fatalf("export after rename: %v (%v)", rows, err)
	}
	var answer string
	if err := f.pool.QueryRow(ctx, `SELECT questionnaire_answer->>'choice' FROM activity_registrations WHERE activity_id=$1`, id).Scan(&answer); err != nil || answer != "Old label" {
		t.Fatalf("saved answer changed: %q (%v)", answer, err)
	}
}
