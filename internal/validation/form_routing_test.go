package validation

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestFormRoutingMetadataRoundTrip(t *testing.T) {
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
	for _, validator := range []string{"customFormValidator", "updateCustomFormValidator"} {
		result, issues := Validate(validator, Object{"formName": marshal("Branching test"), "formSchema": fixture.Schema})
		if len(issues) != 0 {
			t.Fatalf("%s: %v", validator, issues)
		}
		var before, after any
		_ = json.Unmarshal(fixture.Schema, &before)
		_ = json.Unmarshal(result["formSchema"], &after)
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("%s discarded form metadata", validator)
		}
	}
}

func TestInvalidFormRouting(t *testing.T) {
	for _, navigation := range []string{`{"defaultTarget":{"type":"section","sectionId":"missing"}}`, `{"defaultTarget":{"type":"next"},"questionKey":"absent"}`, `{"defaultTarget":{"type":"unknown"}}`} {
		raw := `{"version":2,"fields":[{"id":"one","section_name":"One","fields":[],"navigation":` + navigation + `}]}`
		_, issues := Validate("updateCustomFormValidator", Object{"formSchema": json.RawMessage(raw)})
		if len(issues) == 0 {
			t.Errorf("accepted invalid navigation %s", navigation)
		}
	}
}
