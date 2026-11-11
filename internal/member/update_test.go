package member

import (
	"encoding/json"
	"testing"
)

func TestAddressDetailMergesIntoExtraData(t *testing.T) {
	stored := []byte(`{"preferred_name":"Fixture","address_detail":"Jl. Ganesha 7"}`)
	for body, want := range map[string]string{
		`{"address_detail":"Jl. Dago 1"}`: `{"address_detail":"Jl. Dago 1","preferred_name":"Fixture"}`,
		`{"address_detail":null}`:         `{"address_detail":null,"preferred_name":"Fixture"}`,
		`{"preferred_name":"Other"}`:      `{"address_detail":"Jl. Ganesha 7","preferred_name":"Other"}`,
	} {
		var extra ProfileExtra
		if err := json.Unmarshal([]byte(body), &extra); err != nil {
			t.Fatal(err)
		}
		if got := string(mergeExtra(stored, jsonField(&extra))); got != want {
			t.Errorf("%s: got %s, want %s", body, got, want)
		}
	}
}
