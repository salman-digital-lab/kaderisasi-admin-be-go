package scoring

import (
	"encoding/json"
	"math"
	"testing"
)

func ptr(n float64) *float64 { return &n }
func testDefinition() Definition {
	return Definition{Groups: []Group{{ID: "character", Name: "Karakter", Criteria: []Criterion{{ID: "honesty", Name: "Shiddiq", Maximum: 100, Weight: 1}, {ID: "trust", Name: "Amanah", Maximum: 50, Weight: 3}}}}, Grades: []Grade{{Label: "B", Minimum: 0}, {Label: "A", Minimum: 80}, {Label: "A+", Minimum: 89}}}
}
func TestCalculate(t *testing.T) {
	d := testDefinition()
	for _, tc := range []struct {
		name     string
		scores   map[string]*float64
		total    *float64
		grade    string
		complete bool
		invalid  bool
	}{
		{"normalized weighted", map[string]*float64{"honesty": ptr(100), "trust": ptr(35)}, ptr(77.5), "B", true, false},
		{"zero is complete", map[string]*float64{"honesty": ptr(0), "trust": ptr(0)}, ptr(0), "B", true, false},
		{"blank incomplete", map[string]*float64{"honesty": ptr(0)}, nil, "", false, false},
		{"boundary", map[string]*float64{"honesty": ptr(80), "trust": ptr(40)}, ptr(80), "A", true, false},
		{"decimal", map[string]*float64{"honesty": ptr(86.4), "trust": ptr(50)}, ptr(96.6), "A+", true, false},
		{"overflow", map[string]*float64{"trust": ptr(51)}, nil, "", false, true},
		{"negative", map[string]*float64{"trust": ptr(-1)}, nil, "", false, true},
		{"extra precision", map[string]*float64{"trust": ptr(1.001)}, nil, "", false, true},
		{"nan", map[string]*float64{"trust": ptr(math.NaN())}, nil, "", false, true},
		{"unknown", map[string]*float64{"wrong": ptr(1)}, nil, "", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Calculate(d, Draft{Scores: tc.scores})
			if (err != nil) != tc.invalid {
				t.Fatalf("error=%v", err)
			}
			if tc.invalid {
				return
			}
			if r.Complete != tc.complete || (r.Total == nil) != (tc.total == nil) {
				t.Fatalf("result %#v", r)
			}
			if tc.total != nil && (*r.Total != *tc.total || r.Grade == nil || *r.Grade != tc.grade) {
				t.Fatalf("result %#v", r)
			}
		})
	}
}
func TestRoundingBeforeGrade(t *testing.T) {
	d := testDefinition()
	d.Groups[0].Criteria = d.Groups[0].Criteria[:1]
	d.Groups[0].Criteria[0].Maximum = 3
	d.Grades = []Grade{{Label: "Low", Minimum: 0}, {Label: "High", Minimum: 66.67}}
	r, err := Calculate(d, Draft{Scores: map[string]*float64{"honesty": ptr(2)}})
	if err != nil || *r.Total != 66.67 || *r.Grade != "High" || *r.Criteria[0].Normalized != 66.67 {
		t.Fatalf("rounding %#v %v", r, err)
	}
}
func TestRubricValidation(t *testing.T) {
	for _, mutate := range []func(*Definition){
		func(d *Definition) { d.Groups = nil }, func(d *Definition) { d.Groups[0].Criteria[0].Maximum = 0 }, func(d *Definition) { d.Groups[0].Criteria[0].Weight = -1 }, func(d *Definition) { d.Groups[0].Criteria[1].ID = "honesty" }, func(d *Definition) { d.Grades = []Grade{{Label: "A", Minimum: 80}} }, func(d *Definition) { d.Grades = append(d.Grades, Grade{Label: "Duplicate", Minimum: 80}) },
	} {
		d := testDefinition()
		mutate(&d)
		if ValidateDefinition(d) == nil {
			t.Fatal("accepted invalid rubric")
		}
	}
	d := testDefinition()
	d.Grades = nil
	r, err := Calculate(d, Draft{Scores: map[string]*float64{"honesty": ptr(100), "trust": ptr(50)}})
	if err != nil || r.Grade != nil {
		t.Fatal("optional grades", err)
	}
}
func TestScoringJSONAndState(t *testing.T) {
	d := Data{SchemaVersion: 1, Draft: Draft{Scores: map[string]*float64{"honesty": ptr(0)}, Note: "draft"}}
	setState(&d, Result{Complete: false})
	if d.State != "incomplete" {
		t.Fatal(d.State)
	}
	d.Published = &Snapshot{Draft: Draft{Scores: map[string]*float64{"honesty": ptr(0)}, Note: "published"}}
	setState(&d, Result{Complete: true})
	if d.State != "changed" {
		t.Fatal(d.State)
	}
	raw, _ := json.Marshal(d)
	copy, err := decode(raw)
	if err != nil || *copy.Draft.Scores["honesty"] != 0 || copy.Published.Draft.Note != "published" {
		t.Fatal("round trip", err)
	}
	if data, err := decode(nil); data != nil || err != nil {
		t.Fatal("SQL null")
	}
	if _, err := decode([]byte(`{"schema_version":99}`)); err == nil {
		t.Fatal("unknown version")
	}
}
