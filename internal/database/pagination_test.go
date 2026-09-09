package database

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"
)

func TestLucidPaginationBoundaries(t *testing.T) {
	// Captured from the adopted Lucid paginator through real HTTP comparisons.
	for _, example := range []struct {
		name        string
		total       int64
		page, size  float64
		current     interface{}
		last        interface{}
		lastURL     string
		next, prior *string
	}{
		{"invalid", 3, math.NaN(), math.NaN(), nil, nil, "/?page=NaN", nil, nil},
		{"zero with rows", 3, 0, 0, float64(0), nil, "/?page=Infinity", stringPointer("/?page=1"), nil},
		{"zero without rows", 0, 0, 0, float64(0), nil, "/?page=NaN", nil, nil},
		{"fraction", 3, 1.5, 1.5, 1.5, float64(2), "/?page=2", stringPointer("/?page=2.5"), stringPointer("/?page=1")},
		{"negative empty page", 0, -1, -1, float64(-1), float64(1), "/?page=1", stringPointer("/?page=1"), nil},
	} {
		t.Run(example.name, func(t *testing.T) {
			meta := Meta(example.total, example.page, example.size)
			raw, err := json.Marshal(meta)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]interface{}
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded["current_page"] != example.current || decoded["last_page"] != example.last || meta.LastPageURL != example.lastURL || !reflect.DeepEqual(meta.NextPageURL, example.next) || !reflect.DeepEqual(meta.PreviousPageURL, example.prior) {
				t.Fatalf("unexpected Lucid metadata: %s", raw)
			}
		})
	}
}

func TestKnexPaginationConversion(t *testing.T) {
	limit, offset, err := SQLPage(9007199254740991, 9007199254740991)
	if err != nil || limit == nil || *limit != 9007199254740991 || offset != 8 {
		t.Fatalf("scientific-notation offset conversion: %v %d %v", limit, offset, err)
	}
	limit, offset, err = SQLPage(math.NaN(), math.NaN())
	if err != nil || limit != nil || offset != 0 {
		t.Fatal("invalid numbers must leave SQL limit and offset unset")
	}
	limit, offset, err = SQLPage(1.5, 1.5)
	if err != nil || limit == nil || *limit != 1 || offset != 0 {
		t.Fatal("SQL bounds must use Knex integer truncation")
	}
	if _, _, err := SQLPage(0, 10); err == nil {
		t.Fatal("negative SQL offsets must fail")
	}
}

func stringPointer(value string) *string { return &value }
