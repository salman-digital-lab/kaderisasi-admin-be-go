package database

import (
	"encoding/json"
	"testing"
)

func TestJSONTruthAndDatabaseParameters(t *testing.T) {
	for _, test := range []struct {
		raw       string
		truth     bool
		parameter string
	}{
		{`null`, false, "NULL"}, {`false`, false, "false"}, {`0`, false, "0"}, {`-0`, false, "0"}, {`""`, false, ""},
		{`"0"`, true, "0"}, {`"false"`, true, "false"}, {`[]`, true, "{}"}, {`{}`, true, "{}"},
		{`[1,null,["quoted\"","slash\\"]]`, true, `{"1",NULL,{"quoted\"","slash\\"}}`},
		{`1e2`, true, "100"}, {`{"a": [false,0]}`, true, `{"a":[false,0]}`},
	} {
		t.Run(test.raw, func(t *testing.T) {
			raw := json.RawMessage(test.raw)
			if JSONTruthy(raw) != test.truth {
				t.Fatal("truth test")
			}
			if got := JSONParameter(raw); got != test.parameter {
				t.Fatalf("parameter %q != %q", got, test.parameter)
			}
		})
	}
}
