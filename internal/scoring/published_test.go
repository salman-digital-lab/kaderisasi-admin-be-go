package scoring

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublishedForRegistration(t *testing.T) {
	const raw = `{"schema_version":1,"state":"changed","draft":{"note":"private correction"},"published":{"schema_version":1,"registration_id":7,"activity_id":2,"revision":3,"published_by":99,"published_at":"2026-09-22T00:00:00Z","rubric":{"groups":[],"grades":[],"note":"Rubric note"},"draft":{"note":"Published note","scores":{"a":0}},"result":{"criteria":[{"criterion_id":"a","score":0,"normalized":0,"grade":null}],"total":0,"grade":null,"complete":true}}}`
	result, err := PublishedForRegistration([]byte(raw), 7, 2)
	if err != nil || result == nil || result.Result.Total == nil || *result.Result.Total != 0 || result.Note != "Published note" {
		t.Fatalf("published zero score lost: %+v, %v", result, err)
	}
	serialized, _ := json.Marshal(result)
	for _, private := range []string{"private correction", "published_by", "registration_id", "activity_id", "draft", "scores"} {
		if strings.Contains(string(serialized), private) {
			t.Fatalf("private field leaked: %s", private)
		}
	}
	for name, input := range map[string]string{
		"absent": "", "null": "null", "draft": `{"schema_version":1,"published":null}`,
		"unsupported":       strings.Replace(raw, `"schema_version":1`, `"schema_version":2`, 1),
		"incomplete":        strings.Replace(raw, `"complete":true`, `"complete":false`, 1),
		"other participant": strings.Replace(raw, `"registration_id":7`, `"registration_id":8`, 1),
		"other activity":    strings.Replace(raw, `"activity_id":2`, `"activity_id":3`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			result, err := PublishedForRegistration([]byte(input), 7, 2)
			if err != nil || result != nil {
				t.Fatalf("unpublished result exposed: %+v, %v", result, err)
			}
		})
	}
}
