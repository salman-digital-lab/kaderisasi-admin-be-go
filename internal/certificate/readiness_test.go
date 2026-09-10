package certificate

import (
	"encoding/json"
	"kaderisasi/admin/internal/database"
	"kaderisasi/admin/internal/dbgen"
	"slices"
	"testing"
)

func readyTemplate() database.Object {
	raw := `{"id":10,"name":"Certificate","background_image":null,"template_data":{"backgroundUrl":null,"canvasWidth":800,"canvasHeight":566,"elements":[{"id":"participant-name","type":"variable-text","variable":"{{name}}","x":100,"y":100,"width":600,"height":80}]}}`
	var result database.Object
	_ = json.Unmarshal([]byte(raw), &result)
	return result
}
func TestTemplateReadinessScenarios(t *testing.T) {
	template := readyTemplate()
	if got := CheckReadiness(template); !got.Ready || len(got.Errors) != 0 {
		t.Fatal(got)
	}
	data := rawObject(template["template_data"])
	var elements []database.Object
	_ = json.Unmarshal(data["elements"], &elements)
	extra := rawObject([]byte(`{"id":"email","type":"variable-text","variable":"{{email}}","x":100,"y":200,"width":300,"height":50}`))
	signature := rawObject([]byte(`{"id":"signature","type":"signature","imageUrl":"data:image/png;base64,abc","x":300,"y":300,"width":100,"height":100}`))
	data.Set("elements", append(elements, extra, signature))
	template.Set("template_data", data)
	got := CheckReadiness(template)
	if got.Ready || !slices.Contains(got.Errors, "UNSUPPORTED_VARIABLE") || !slices.Contains(got.Errors, "ELEMENT_MUST_USE_MANAGED_ASSET") {
		t.Fatal(got)
	}
	elements[0].Set("visible", false)
	data.Set("elements", elements)
	template.Set("template_data", data)
	if !slices.Contains(CheckReadiness(template).Errors, "PARTICIPANT_NAME_VARIABLE_REQUIRED") {
		t.Fatal("hidden participant name")
	}
	lifecycle := TemplateView(dbgen.CertificateTemplate{LifecycleStatus: "draft", Version: 2}, 0, 0)
	if lifecycle.Status != "draft" || lifecycle.IsActive || lifecycle.PublishedAt != nil || lifecycle.ArchivedAt != nil {
		t.Fatal(lifecycle)
	}
}
func TestTemplateAssetKeys(t *testing.T) {
	for _, value := range []string{"certificate/templates/10/assets/a.webp", "https://storage.example/bucket/certificate/templates/10/assets/a.webp"} {
		key, err := AssetKey(value, 10)
		if err != nil || key != "certificate/templates/10/assets/a.webp" {
			t.Fatal(key, err)
		}
	}
	for _, value := range []string{"certificate/templates/11/assets/a.webp", "certificate/templates/10/../secret", "certificate/templates/10/assets/a.webp?x=y", "data:image/png;base64,abc", "https://storage.example/certificate/templates/10/%2e%2e/secret"} {
		if _, err := AssetKey(value, 10); err == nil {
			t.Error("unsafe key", value)
		}
	}
}
