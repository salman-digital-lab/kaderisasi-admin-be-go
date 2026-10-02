package validation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestActivityOptionalFeaturesSurviveValidation(t *testing.T) {
	config := `{"custom_selection_status":[],"mandatory_profile_data":[],"additional_questionnaire":[],"optional_features":{"scoring":true,"courses":false}}`
	for _, name := range []string{"activityValidator", "updateActivityValidator"} {
		output, issues := Validate(name, Object{"name": json.RawMessage(`"Kegiatan"`), "activity_type": json.RawMessage(`1`), "additional_config": json.RawMessage(config)})
		if len(issues) > 0 {
			t.Fatalf("%s: %v", name, issues)
		}
		var got struct {
			OptionalFeatures struct {
				Scoring *bool `json:"scoring"`
				Courses *bool `json:"courses"`
			} `json:"optional_features"`
		}
		if err := json.Unmarshal(output["additional_config"], &got); err != nil {
			t.Fatal(err)
		}
		if got.OptionalFeatures.Scoring == nil || !*got.OptionalFeatures.Scoring || got.OptionalFeatures.Courses == nil || *got.OptionalFeatures.Courses {
			t.Fatalf("%s dropped optional features: %s", name, output["additional_config"])
		}
		invalid := strings.Replace(config, `"scoring":true`, `"scoring":"yes"`, 1)
		if _, issues := Validate(name, Object{"name": json.RawMessage(`"Kegiatan"`), "activity_type": json.RawMessage(`1`), "additional_config": json.RawMessage(invalid)}); len(issues) == 0 {
			t.Fatalf("%s accepted a non-boolean feature flag", name)
		}
	}
}
