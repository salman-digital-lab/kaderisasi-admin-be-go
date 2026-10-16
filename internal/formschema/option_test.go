package formschema

import (
	"encoding/json"
	"testing"
)

func TestChoiceAnswerTextShowsOptionLabels(t *testing.T) {
	var field Field
	if err := json.Unmarshal([]byte(`{"key":"division","label":"Pilihan Divisi 1","type":"select","required":true,"options":[
		{"label":"Medik","value":"4f0c7a52-9b1e-4d3a-8f6e-2a1b3c4d5e6f"},
		{"label":"Konsumsi, Operasional, dan Logistik (KOL)","value":"Konsumsi,"},
		{"label":"Acara"},
		{"label":"Nol","value":0}
	]}`), &field); err != nil {
		t.Fatal(err)
	}
	for raw, want := range map[string]string{
		`"4f0c7a52-9b1e-4d3a-8f6e-2a1b3c4d5e6f"`: "Medik",
		`"Konsumsi,"`:                            "Konsumsi, Operasional, dan Logistik (KOL)",
		`"Acara"`:                                "Acara",
		`0`:                                      "Nol",
		`"Lainnya"`:                              "Lainnya",
		`["Konsumsi,","Lainnya","4f0c7a52-9b1e-4d3a-8f6e-2a1b3c4d5e6f"]`: "Konsumsi, Operasional, dan Logistik (KOL), Lainnya, Medik",
	} {
		got, ok := field.ChoiceAnswerText(json.RawMessage(raw))
		if !ok || got != want {
			t.Fatalf("%s: got %q, %v; want %q", raw, got, ok, want)
		}
	}
	for _, raw := range []string{`null`, `{"a":1}`, `[{"a":1}]`} {
		if _, ok := field.ChoiceAnswerText(json.RawMessage(raw)); ok {
			t.Fatalf("%s: expected raw fallback", raw)
		}
	}
	field.Type = "text"
	if _, ok := field.ChoiceAnswerText(json.RawMessage(`"Konsumsi,"`)); ok {
		t.Fatal("text field mapped to an option label")
	}
}
