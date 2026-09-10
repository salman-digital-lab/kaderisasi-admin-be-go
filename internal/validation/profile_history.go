package validation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Keep the historical Vine schema for compatibility fixtures; the active
// profile endpoint accepts partial education records produced by web-be/imports.
func memberProfileRule(base Rule) Rule {
	var fields map[string]Rule
	_ = json.Unmarshal(base.Args[0], &fields)
	var education map[string]Rule
	educationArray := fields["education_history"]
	var educationObject Rule
	_ = json.Unmarshal(educationArray.Args[0], &educationObject)
	_ = json.Unmarshal(educationObject.Args[0], &education)
	for key, rule := range education {
		rule.Chain = append(rule.Chain, Constraint{Method: "optional", Args: []json.RawMessage{}})
		if rule.Kind == "string" {
			rule.Chain = append(rule.Chain, Constraint{Method: "trim", Args: []json.RawMessage{}})
		}
		if key == "intake_year" {
			rule.Chain = append(rule.Chain, historyYearConstraints()...)
		}
		education[key] = rule
	}
	educationObject.Args[0] = orderedRuleUpdates(educationObject.Args[0], education)
	educationArray.Args[0] = marshal(educationObject)
	fields["education_history"] = educationArray

	workArray := fields["work_history"]
	var workObject Rule
	_ = json.Unmarshal(workArray.Args[0], &workObject)
	var work map[string]Rule
	_ = json.Unmarshal(workObject.Args[0], &work)
	for key, rule := range work {
		if rule.Kind == "string" {
			rule.Chain = append(rule.Chain, Constraint{Method: "trim"}, Constraint{Method: "minLength", Args: []json.RawMessage{marshal(1)}})
		} else {
			rule.Chain = append(rule.Chain, historyYearConstraints()...)
		}
		work[key] = rule
	}
	workObject.Args[0] = orderedRuleUpdates(workObject.Args[0], work)
	workArray.Args[0] = marshal(workObject)
	fields["work_history"] = workArray
	base.Args[0] = orderedRuleUpdates(base.Args[0], fields)
	return base
}

func orderedRuleUpdates(original json.RawMessage, fields map[string]Rule) json.RawMessage {
	var result bytes.Buffer
	result.WriteByte('{')
	for index, key := range orderedKeys(original) {
		if index > 0 {
			result.WriteByte(',')
		}
		result.Write(marshal(key))
		result.WriteByte(':')
		result.Write(marshal(fields[key]))
	}
	result.WriteByte('}')
	return result.Bytes()
}

func historyYearConstraints() []Constraint {
	return []Constraint{
		{Method: "withoutDecimals"},
		{Method: "range", Args: []json.RawMessage{marshal([]int{1900, time.Now().Year() + 10})}},
	}
}

func workHistoryIssues(output Object) []Issue {
	var entries []Object
	_ = json.Unmarshal(output["work_history"], &entries)
	for index, entry := range entries {
		if entry.Has("start_year") && entry.Has("end_year") && entry.Number("end_year") < entry.Number("start_year") {
			return issue(fmt.Sprintf("work_history.%d.end_year", index), "workYearRange", "Tahun selesai tidak boleh lebih kecil dari tahun mulai")
		}
	}
	return nil
}
