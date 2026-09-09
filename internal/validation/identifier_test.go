package validation

import (
	"encoding/json"
	"testing"
)

func TestIdentifierNeverNarrowsIntoAnotherRecord(t *testing.T) {
	for _, raw := range []string{"1.5", "2147483648", "4294967297", "9007199254740991", "-2147483649"} {
		if id := (Object{"id": json.RawMessage(raw)}).ID("id"); id != 0 {
			t.Fatalf("%s narrowed into record %d", raw, id)
		}
	}
	for _, raw := range []string{"1", "1.0", "1e0"} {
		if id := (Object{"id": json.RawMessage(raw)}).ID("id"); id != 1 {
			t.Fatalf("valid numeric ID %s lost", raw)
		}
	}
}
