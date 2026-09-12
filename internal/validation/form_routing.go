package validation

import (
	"bytes"
	"encoding/json"
	"kaderisasi/admin/internal/formschema"
)

// Extend the active form contract without changing the historical Vine fixtures.
func formRoutingRule(base Rule) Rule {
	var root map[string]Rule
	_ = json.Unmarshal(base.Args[0], &root)
	schema := root["formSchema"]
	var schemaFields map[string]Rule
	_ = json.Unmarshal(schema.Args[0], &schemaFields)
	sections := schemaFields["fields"]
	var section Rule
	_ = json.Unmarshal(sections.Args[0], &section)
	str := Rule{Kind: "string", Args: []json.RawMessage{}}
	destination := Rule{Kind: "object", Args: []json.RawMessage{marshal(map[string]Rule{
		"type":      {Kind: "enum", Args: []json.RawMessage{marshal([]string{"next", "section", "submit"})}},
		"sectionId": optionalFormRule(str),
	})}}
	route := Rule{Kind: "object", Args: []json.RawMessage{marshal(map[string]Rule{
		"optionValue": str, "target": destination,
	})}}
	navigation := Rule{Kind: "object", Args: []json.RawMessage{marshal(map[string]Rule{
		"defaultTarget": destination,
		"questionKey":   optionalFormRule(str),
		"routes":        optionalFormRule(Rule{Kind: "array", Args: []json.RawMessage{marshal(route)}}),
	})}}
	section = appendFormRules(section, map[string]Rule{
		"id": optionalFormRule(str), "description": optionalFormRule(str), "navigation": optionalFormRule(navigation),
	})
	sections.Args[0] = marshal(section)
	schemaFields["fields"] = sections
	schema.Args[0] = orderedRuleUpdates(schema.Args[0], schemaFields)
	schema = appendFormRules(schema, map[string]Rule{"version": optionalFormRule(Rule{Kind: "enum", Args: []json.RawMessage{marshal([]int{2})}})})
	root["formSchema"] = schema
	base.Args[0] = orderedRuleUpdates(base.Args[0], root)
	return base
}

func optionalFormRule(rule Rule) Rule {
	rule.Chain = append(rule.Chain, Constraint{Method: "optional"})
	return rule
}

func appendFormRules(rule Rule, extra map[string]Rule) Rule {
	raw := bytes.TrimSpace(rule.Args[0])
	addition := marshal(extra)
	combined := append([]byte{}, raw[:len(raw)-1]...)
	if len(raw) > 2 {
		combined = append(combined, ',')
	}
	combined = append(combined, addition[1:]...)
	rule.Args[0] = combined
	return rule
}

func formRoutingIssues(output Object) []Issue {
	raw, exists := output["formSchema"]
	if !exists {
		return nil
	}
	var schema formschema.Schema
	if json.Unmarshal(raw, &schema) != nil || !formschema.ValidRouting(schema) {
		return issue("formSchema", "formRouting", "Konfigurasi alur formulir tidak valid. Periksa tujuan bagian dan pilihan jawaban.")
	}
	return nil
}
