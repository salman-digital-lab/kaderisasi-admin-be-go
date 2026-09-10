package validation

import (
	"encoding/json"
	"testing"
)

func TestMemberProfileHistoryValidation(t *testing.T) {
	for _, input := range []string{
		`{"education_history":[{"major":"Physics","intake_year":null}],"work_history":[{"job_title":" Engineer ","company":" Company ","start_year":"2021","end_year":""}]}`,
		`{"education_history":[],"work_history":[]}`,
		`{"name":"Fixture"}`,
	} {
		var request Object
		_ = json.Unmarshal([]byte(input), &request)
		output, issues := Validate("memberProfileUpdateValidator", request)
		if len(issues) > 0 {
			t.Fatalf("%s: %+v", input, issues)
		}
		if request.Has("education_history") != output.Has("education_history") || request.Has("work_history") != output.Has("work_history") {
			t.Fatalf("omitted/empty history changed: %s", marshal(output))
		}
	}
	for _, input := range []string{
		`{"education_history":[{"intake_year":2017.5}]}`,
		`{"education_history":[{"intake_year":0}]}`,
		`{"work_history":[{"job_title":"Engineer","company":"Company","start_year":2025,"end_year":2021}]}`,
		`{"work_history":[{"job_title":" ","company":"Company"}]}`,
		`{"work_history":[{"job_title":"Engineer","company":"Company","start_year":2021.5}]}`,
		`{"education_history":[null]}`,
	} {
		var request Object
		_ = json.Unmarshal([]byte(input), &request)
		if _, issues := Validate("memberProfileUpdateValidator", request); len(issues) == 0 {
			t.Fatalf("accepted %s", input)
		}
	}
}
