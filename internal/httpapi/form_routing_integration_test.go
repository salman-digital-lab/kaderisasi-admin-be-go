//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestFormRoutingRoundTrip(t *testing.T) {
	f := newHTTPFixture(t)
	raw, err := os.ReadFile("../formschema/routing.fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Schema json.RawMessage `json:"schema"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	created := objectData(t, f.call("POST", "/v2/custom-forms", map[string]interface{}{"formName": "Routing contract", "formSchema": fixture.Schema}, f.token, 201))
	id := created.ID("id")
	t.Cleanup(func() {
		if _, err := f.pool.Exec(context.Background(), "DELETE FROM custom_forms WHERE id=$1", id); err != nil {
			t.Error(err)
		}
	})
	path := fmt.Sprintf("/v2/custom-forms/%d", id)
	assertSchema := func(raw json.RawMessage) {
		t.Helper()
		var expected, actual interface{}
		_ = json.Unmarshal(fixture.Schema, &expected)
		if err := json.Unmarshal(raw, &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(expected, actual) {
			t.Fatal("routing metadata changed during round trip")
		}
	}
	assertSchema(created["form_schema"])
	updated := objectData(t, f.call("PUT", path, map[string]interface{}{"formName": "Renamed routing contract", "formSchema": fixture.Schema}, f.token, 200))
	assertSchema(updated["form_schema"])
	shown := objectData(t, f.call("GET", path, nil, f.token, 200))
	assertSchema(shown["form_schema"])
	for _, invalid := range []string{
		`{"version":2,"fields":[{"id":"one","section_name":"One","fields":[],"navigation":{"defaultTarget":{"type":"section","sectionId":"missing"}}}]}`,
		`{"version":2,"fields":[{"id":"one","section_name":"One","fields":[],"navigation":{"defaultTarget":{"type":"submit"},"questionKey":"gender"}}]}`,
		`{"version":2,"fields":[{"id":"one","section_name":"One","fields":[]},{"id":"one","section_name":"Two","fields":[]}]}`,
	} {
		f.call("PUT", path, map[string]interface{}{"formSchema": json.RawMessage(invalid)}, f.token, 500)
	}
	shown = objectData(t, f.call("GET", path, nil, f.token, 200))
	assertSchema(shown["form_schema"])
}
