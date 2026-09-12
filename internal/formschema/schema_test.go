package formschema

import (
	"encoding/json"
	"os"
	"testing"
)

func fixtureSchema(t *testing.T) Schema {
	t.Helper()
	raw, err := os.ReadFile("routing.fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Schema Schema `json:"schema"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.Schema
}

func TestRoutingContract(t *testing.T) {
	schema := fixtureSchema(t)
	raw, _ := json.Marshal(schema)
	if !ValidSchema(raw) {
		t.Fatal("valid branching schema rejected")
	}
	for _, target := range []string{"profile", "choice", "missing"} {
		invalid := fixtureSchema(t)
		invalid.Fields[1].Navigation.Routes[0].Target.SectionID = target
		if ValidRouting(invalid) {
			t.Errorf("accepted invalid destination %s", target)
		}
	}
	invalid := fixtureSchema(t)
	invalid.Fields[1].Fields[0].Disabled = new(true)
	if ValidRouting(invalid) {
		t.Fatal("accepted disabled routing question")
	}
	invalid = fixtureSchema(t)
	invalid.Fields[2].Fields[0].Key = invalid.Fields[1].Fields[0].Key
	if ValidRouting(invalid) {
		t.Fatal("accepted duplicate question key")
	}
	invalid = fixtureSchema(t)
	invalid.Fields[2].ID = invalid.Fields[1].ID
	if ValidRouting(invalid) {
		t.Fatal("accepted duplicate section ID")
	}
	legacy := fixtureSchema(t)
	legacy.Version = nil
	for i := range legacy.Fields {
		legacy.Fields[i].ID = nil
		legacy.Fields[i].Navigation = nil
	}
	raw, _ = json.Marshal(legacy)
	if !ValidSchema(raw) {
		t.Fatal("legacy schema rejected")
	}
}

func TestNormalizedRoutingValues(t *testing.T) {
	for raw, expected := range map[string]string{`1000000`: "1000000", `1e-7`: "1e-7", `-0`: "0", `false`: "false", `0`: "0", `""`: "Detail"} {
		schema := fixtureSchema(t)
		(*schema.Fields[1].Fields[0].Options)[0].Value = json.RawMessage(raw)
		schema.Fields[1].Navigation.Routes = append(schema.Fields[1].Navigation.Routes, AnswerRoute{OptionValue: expected, Target: Destination{Type: "submit"}})
		if !ValidRouting(schema) {
			t.Errorf("rejected normalized option %s -> %s", raw, expected)
		}
	}
}
