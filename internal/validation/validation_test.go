package validation

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestVineCompatibility(t *testing.T) {
	raw, err := os.ReadFile("testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name, Label string
		Input       Object
		Output      Object
		Issues      []Issue
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name+"/"+f.Label, func(t *testing.T) {
			// Role choices intentionally changed after the captured Vine baseline.
			// Keep every other error property and validation contract unchanged.
			if f.Label == "invalid-role_code" {
				f.Issues[0].Meta = map[string]interface{}{"choices": []interface{}{
					"super_admin", "admin", "activity_manager", "achievement_manager",
					"club_manager", "konselor",
				}}
			}
			output, issues := Validate(f.Name, f.Input)
			if len(issues) != len(f.Issues) {
				t.Fatalf("issues got %+v want %+v", issues, f.Issues)
			}
			if len(issues) > 0 {
				if !reflect.DeepEqual(issues, f.Issues) {
					t.Fatalf("issues got %+v want %+v", issues, f.Issues)
				}
				return
			}
			a, _ := json.Marshal(output)
			b, _ := json.Marshal(f.Output)
			var decodedA, decodedB interface{}
			_ = json.Unmarshal(a, &decodedA)
			_ = json.Unmarshal(b, &decodedB)
			if !reflect.DeepEqual(decodedA, decodedB) {
				t.Fatalf("output got %s want %s", a, b)
			}
		})
	}
}
