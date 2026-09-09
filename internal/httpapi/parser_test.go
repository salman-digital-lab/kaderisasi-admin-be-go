package httpapi

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestInstalledParserCompatibility(t *testing.T) {
	raw, err := os.ReadFile("../../tests/fixtures/parser.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Forms []struct {
			Raw       string
			EmptyNull bool
			Expected  map[string]any
		}
		JSON []struct {
			Raw     string
			Message string
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, test := range fixture.Forms {
		t.Run("form/"+test.Raw, func(t *testing.T) {
			actual := parseFields(test.Raw, test.EmptyNull)
			if !reflect.DeepEqual(actual, test.Expected) {
				t.Errorf("got %#v; want %#v", actual, test.Expected)
			}
		})
	}
	for _, test := range fixture.JSON {
		t.Run("json/"+test.Raw, func(t *testing.T) {
			var data json.RawMessage
			err := json.Unmarshal([]byte(test.Raw), &data)
			if err == nil {
				t.Fatal("invalid JSON accepted")
			}
			actual := jsonDiagnostic([]byte(test.Raw), err)
			if actual != test.Message {
				t.Errorf("got %q; want %q; Go syntax: %v", actual, test.Message, err)
			}
		})
	}
}
