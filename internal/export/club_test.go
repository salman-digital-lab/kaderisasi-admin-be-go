package export

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestClubExportQuestionOrder(t *testing.T) {
	schema := []byte(`{"fields":[{"section_name":"profile_data","fields":[{"key":"name","label":"Nama"}]},{"section_name":"Motivasi","fields":[{"key":"motivation","label":"Motivasi"},{"key":"division","label":"Divisi"}]}]}`)
	got := ClubQuestions(schema, [][]byte{[]byte(`{"motivation":"Belajar","old_question":"Jawaban lama"}`)})
	want := []Question{{"motivation", "Motivasi"}, {"division", "Divisi"}, {"old_question", "Old question"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("question order %v", got)
	}
}
func TestClubExportValues(t *testing.T) {
	for _, test := range []struct {
		raw  string
		want interface{}
	}{{"false", "Tidak"}, {"0", float64(0)}, {`["A","B"]`, "A, B"}, {`{"note":"A"}`, `{"note":"A"}`}, {"null", ""}} {
		if got := ClubAnswer(json.RawMessage(test.raw)); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s = %#v", test.raw, got)
		}
	}
}
