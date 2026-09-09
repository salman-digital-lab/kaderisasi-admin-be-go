package domain

import (
	"encoding/json"
	"testing"
)

func TestPatchPresence(t *testing.T) {
	type patch struct {
		Role   Optional[string] `json:"role,omitzero"`
		Active Optional[bool]   `json:"active,omitzero"`
	}
	for _, raw := range []string{`{}`, `{"role":null}`, `{"role":"super_admin","active":false}`} {
		var input patch
		if err := json.Unmarshal([]byte(raw), &input); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(input)
		if err != nil || string(encoded) != raw {
			t.Fatalf("PATCH state lost: %s -> %s (%v)", raw, encoded, err)
		}
	}
}
