package validation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFormFileSettingsSurviveValidation(t *testing.T) {
	for _, kind := range []string{"pdf", "image", "pdf_or_image"} {
		raw := `{"version":2,"settings":{"accessMode":"public"},"fields":[{"id":"files","section_name":"Berkas","fields":[{"key":"proof","label":"Bukti","type":"file","required":true,"file":{"accept":"` + kind + `","maxFiles":5,"maxSizeMB":10}}]}]}`
		output, issues := Validate("updateCustomFormValidator", Object{"formSchema": json.RawMessage(raw)})
		if len(issues) > 0 {
			t.Fatalf("%s: %v", kind, issues)
		}
		if !strings.Contains(string(output["formSchema"]), `"maxFiles":5`) || !strings.Contains(string(output["formSchema"]), `"accessMode":"public"`) {
			t.Fatal(string(output["formSchema"]))
		}
		for _, invalid := range []string{strings.Replace(raw, `"maxFiles":5`, `"maxFiles":6`, 1), strings.Replace(raw, `"maxSizeMB":10`, `"maxSizeMB":11`, 1), strings.Replace(raw, `"maxFiles":5`, `"maxFiles":1.5`, 1)} {
			if _, issues := Validate("updateCustomFormValidator", Object{"formSchema": json.RawMessage(invalid)}); len(issues) == 0 {
				t.Fatal("invalid limits accepted")
			}
		}
	}
}
