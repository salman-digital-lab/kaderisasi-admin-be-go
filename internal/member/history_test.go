package member

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"testing"
)

func TestHistoryNormalization(t *testing.T) {
	for _, degree := range []string{"high_school", "diploma"} {
		raw := []byte(`[{"degree":"` + degree + `","institution":"School"}]`)
		var entries []EducationEntry
		if err := json.Unmarshal(NormalizeEducationHistory(raw), &entries); err != nil || len(entries) != 1 || entries[0].Degree == nil || *entries[0].Degree != degree {
			t.Fatalf("degree lost after normalization: %s", raw)
		}
	}
	raw, _ := json.Marshal(`[null,{"degree":"bachelor","institution":" ITB ","intake_year":"2017"}]`)
	view := ProfileView(dbgen.Profile{EducationHistory: raw, WorkHistory: []byte(`[null,{"job_title":" Engineer ","company":"Company","start_year":"2021","end_year":null}]`)})
	if string(view.EducationHistory) != `[{"degree":"bachelor","institution":"ITB","faculty":"","major":"","intake_year":2017}]` {
		t.Fatalf("education: %s", view.EducationHistory)
	}
	if string(view.WorkHistory) != `[{"job_title":"Engineer","company":"Company","start_year":2021}]` {
		t.Fatalf("work: %s", view.WorkHistory)
	}
	for _, raw := range []string{`{}`, `"invalid"`, `5`} {
		if string(NormalizeEducationHistory([]byte(raw))) != "[]" || string(NormalizeWorkHistory([]byte(raw))) != "[]" {
			t.Fatalf("invalid history %s", raw)
		}
	}
	if string(NormalizeEducationHistory([]byte("null"))) != "null" {
		t.Fatal("null history changed")
	}
}
