package activity

import (
	"encoding/json"
	"kaderisasi/admin/internal/dbgen"
	"strings"
	"testing"
)

func TestRegistrationViewHidesScoring(t *testing.T) {
	row := dbgen.ActivityRegistration{ID: 1, ScoringData: []byte(`{"draft":{"note":"private"}}`)}
	for _, view := range []interface{}{RegistrationView(row), CreatedRegistrationView(row)} {
		body, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(body), "scoring_data") || strings.Contains(string(body), "private") {
			t.Fatalf("leaked draft: %s", body)
		}
	}
}
